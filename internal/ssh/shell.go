package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Shell markers used for framing commands in shell mode.
const (
	shellReadyMarkerPrefix = "__MCP_READY__"
	shellBeginMarkerPrefix = "__MCP_BEGIN__"
	shellEndMarkerPrefix   = "__MCP_END__"
	shellMarkerSuffix      = "__"
	shellEndRCPrefix       = "__RC__"
)

// shellSession holds the state of a persistent shell session.
type shellSession struct {
	mu       sync.Mutex
	session  *ssh.Session
	stdin    io.WriteCloser
	stdout   io.Reader
	buf      bytes.Buffer
	ready    bool
	cmdQueue chan *shellCmd
}

type shellCmd struct {
	script    string
	commandID string
	result    chan *shellCmdResult
}

type shellCmdResult struct {
	result *ExecResult
	err    error
}

// ShellRunner manages a persistent shell session for a single SSH connection.
// Commands are serialized via a command queue.
type ShellRunner struct {
	client  SSHClient
	session *shellSession
	ready   bool
	mu      sync.Mutex
}

// NewShellRunner creates a new ShellRunner for the given client.
func NewShellRunner(client SSHClient) *ShellRunner {
	return &ShellRunner{client: client}
}

// EnsureReady opens the shell session if not already open and waits for
// the shell to be ready.
func (r *ShellRunner) EnsureReady(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ready && r.session != nil {
		return nil
	}

	srv := r.client.Config()
	session, err := r.client.NewSession()
	if err != nil {
		return NewToolError(CodeSSHConnectionFailed,
			fmt.Sprintf("create shell session: %v", err), true)
	}

	// Request PTY for shell mode. Use TERM=dumb to minimize escape sequences
	// and set ECHO=0 to suppress input echo.
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("dumb", 80, 200, modes); err != nil {
		session.Close()
		return NewToolError(CodeSSHConnectionFailed,
			fmt.Sprintf("request PTY for shell: %v", err), true)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return NewToolError(CodeSSHConnectionFailed,
			fmt.Sprintf("get stdin pipe: %v", err), true)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return NewToolError(CodeSSHConnectionFailed,
			fmt.Sprintf("get stdout pipe: %v", err), true)
	}

	if err := session.Shell(); err != nil {
		session.Close()
		return NewToolError(CodeSSHConnectionFailed,
			fmt.Sprintf("start shell: %v", err), true)
	}

	ss := &shellSession{
		session:  session,
		stdin:    stdin,
		stdout:   stdout,
		cmdQueue: make(chan *shellCmd, 16),
	}

	// Start reading stdout into the buffer immediately, before sending
	// the ready probe. Without this, waitForMarker would block forever
	// because the buffer would never be filled.
	go r.readLoop(ss)

	// Wait for shell ready
	readyID := generateMarkerID("ready")
	readyMarker := shellReadyMarkerPrefix + readyID + shellMarkerSuffix

	// Send ready probe. Set PS1 to empty to suppress prompt output
	// that would interfere with marker scanning.
	probeCmd := fmt.Sprintf("PS1=''; PS2=''; printf '%s\\n'\n", readyMarker)
	if _, err := stdin.Write([]byte(probeCmd)); err != nil {
		session.Close()
		return NewToolError(CodeSSHConnectionFailed,
			fmt.Sprintf("send ready probe: %v", err), true)
	}

	// Wait for ready marker in output
	readyTimeout := srv.GetShellReadyTimeout()
	readyCtx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()

	if err := waitForMarker(readyCtx, ss, readyMarker); err != nil {
		session.Close()
		return NewToolError(CodeSSHConnectionFailed,
			fmt.Sprintf("shell ready timeout: %v", err), true)
	}

	// Start the command processing goroutine
	r.session = ss
	r.ready = true
	go r.processQueue(ss)

	return nil
}

// processQueue reads commands from the queue, sends them to the shell,
// and collects their output. The readLoop must already be running.
func (r *ShellRunner) processQueue(ss *shellSession) {
	for cmd := range ss.cmdQueue {
		result := r.executeOne(ss, cmd)
		cmd.result <- result
	}
}

// readLoop continuously reads from stdout into the session buffer.
func (r *ShellRunner) readLoop(ss *shellSession) {
	buf := make([]byte, 4096)
	for {
		n, err := ss.stdout.Read(buf)
		if n > 0 {
			ss.mu.Lock()
			ss.buf.Write(buf[:n])
			ss.mu.Unlock()
		}
		if err != nil {
			if err != io.EOF {
				slog.Debug("shell stdout read ended", "error", err)
			}
			return
		}
	}
}

// executeOne sends a command to the shell and waits for its output.
func (r *ShellRunner) executeOne(ss *shellSession, cmd *shellCmd) *shellCmdResult {
	srv := r.client.Config()
	beginMarker := shellBeginMarkerPrefix + cmd.commandID + shellMarkerSuffix
	endMarker := shellEndMarkerPrefix + cmd.commandID + shellEndRCPrefix

	// Send the command script
	if _, err := ss.stdin.Write([]byte(cmd.script)); err != nil {
		return &shellCmdResult{
			err: NewToolError(CodeSSHConnectionFailed,
				fmt.Sprintf("write to shell: %v", err), true),
		}
	}

	// Wait for the end marker in the output
	timeout := srv.GetCommandTimeout()
	deadline := time.Now().Add(timeout)
	start := time.Now()

	for time.Now().Before(deadline) {
		ss.mu.Lock()
		bufStr := ss.buf.String()
		ss.mu.Unlock()

		// Look for end marker
		endIdx := strings.Index(bufStr, endMarker)
		if endIdx >= 0 {
			// Parse exit code from after the end marker
			afterEnd := bufStr[endIdx+len(endMarker):]
			exitCode := parseExitCode(afterEnd)

			// Extract output between begin and end markers
			beginIdx := strings.Index(bufStr, beginMarker)
			output := ""
			if beginIdx >= 0 {
				startOfOutput := beginIdx + len(beginMarker)
				// Skip the newline after begin marker
				if startOfOutput < len(bufStr) && bufStr[startOfOutput] == '\n' {
					startOfOutput++
				}
				output = bufStr[startOfOutput:endIdx]
				// Trim trailing newline before end marker
				output = strings.TrimSuffix(output, "\n")
				output = strings.TrimSuffix(output, "\r")
			}

			// Remove consumed data from buffer
			afterMarker := endIdx + len(endMarker)
			// Skip the exit code and trailing newline
			if idx := strings.IndexByte(bufStr[afterMarker:], '\n'); idx >= 0 {
				afterMarker += idx + 1
			}
			ss.mu.Lock()
			remaining := ss.buf.String()[afterMarker:]
			ss.buf.Reset()
			ss.buf.WriteString(remaining)
			ss.mu.Unlock()

			// Strip ANSI escape sequences
			output = stripANSI(output)

			maxOutput := srv.GetMaxOutputBytes()
			truncated := int64(len(output)) > maxOutput
			if truncated {
				output = output[:maxOutput]
			}

			return &shellCmdResult{
				result: &ExecResult{
					Stdout:    output,
					ExitCode:  exitCode,
					Duration:  time.Since(start).String(),
					Truncated: truncated,
				},
			}
		}

		time.Sleep(10 * time.Millisecond)
	}

	return &shellCmdResult{
		err: NewToolError(CodeCommandTimeout, "shell command timed out", true),
	}
}

// ExecCommand runs a command in shell mode.
func (r *ShellRunner) ExecCommand(ctx context.Context, command string, directory string, timeout time.Duration) (*ExecResult, error) {
	if err := r.EnsureReady(ctx); err != nil {
		return nil, err
	}

	srv := r.client.Config()

	// Validate against policy
	if pol := r.client.Policy(); pol != nil {
		allowed, reason := pol.Validate(command)
		if !allowed {
			return nil, NewToolError(CodeCommandValidationFailed, reason, false)
		}
	}

	commandID := generateMarkerID("command")
	script := buildShellCommandScript(commandID, command, directory, srv.CommandTemplate)

	resultCh := make(chan *shellCmdResult, 1)
	cmd := &shellCmd{
		script:    script,
		commandID: commandID,
		result:    resultCh,
	}

	r.session.cmdQueue <- cmd

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return nil, res.err
		}
		return res.result, nil
	}
}

// Close closes the shell session.
func (r *ShellRunner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session != nil {
		close(r.session.cmdQueue)
		r.session.stdin.Close()
		r.session.session.Close()
		r.session = nil
		r.ready = false
	}
	return nil
}

// buildShellCommandScript creates the shell script that wraps a command
// with begin/end markers for output framing.
func buildShellCommandScript(commandID, cmdString, directory, commandTemplate string) string {
	beginMarker := shellBeginMarkerPrefix + commandID + shellMarkerSuffix
	endMarker := shellEndMarkerPrefix + commandID + shellEndRCPrefix

	commandBody := fmt.Sprintf("{ %s; }", cmdString)
	if directory != "" {
		commandBody = fmt.Sprintf("cd -- %s && { %s; }", shellQuote(directory), cmdString)
	}

	if commandTemplate != "" {
		commandBody = applyCommandTemplate(commandTemplate, commandBody, "")
	}

	return fmt.Sprintf("printf '%s\\n'\n%s\n__mcp_rc=$?\nprintf '\\n%s%%s__\\n' \"$__mcp_rc\"\n",
		beginMarker, commandBody, endMarker)
}

// generateMarkerID generates a unique marker ID for command framing.
func generateMarkerID(prefix string) string {
	return fmt.Sprintf("%s%d%d", prefix, time.Now().UnixNano(), rand.Intn(10000))
}

// waitForMarker waits for the given marker to appear in the session buffer.
func waitForMarker(ctx context.Context, ss *shellSession, marker string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ss.mu.Lock()
		bufStr := ss.buf.String()
		ss.mu.Unlock()

		if idx := strings.Index(bufStr, marker); idx >= 0 {
			// Consume up to and including the marker
			ss.mu.Lock()
			remaining := bufStr[idx+len(marker):]
			// Skip trailing newline
			if strings.HasPrefix(remaining, "\n") {
				remaining = remaining[1:]
			} else if strings.HasPrefix(remaining, "\r\n") {
				remaining = remaining[2:]
			}
			ss.buf.Reset()
			ss.buf.WriteString(remaining)
			ss.mu.Unlock()
			return nil
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// parseExitCode parses the exit code from the text after the end marker.
// The format is "{exitCode}__\n".
func parseExitCode(s string) int {
	// Find the closing "__"
	idx := strings.Index(s, shellMarkerSuffix)
	if idx < 0 {
		return -1
	}
	codeStr := strings.TrimSpace(s[:idx])
	if codeStr == "" {
		return -1
	}
	var code int
	if _, err := fmt.Sscanf(codeStr, "%d", &code); err != nil {
		return -1
	}
	return code
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	// Strip CSI sequences: ESC [ ... letter
	s = strings.ReplaceAll(s, "\x1b[0m", "")
	// General CSI pattern
	var result strings.Builder
	result.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Skip until we find a letter
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++ // skip the letter
			}
			i = j
		} else if s[i] == 0x1b && i+1 < len(s) && s[i+1] == ']' {
			// OSC sequence: skip until BEL or ST
			j := i + 2
			for j < len(s) && s[j] != 0x07 && s[j] != 0x1b {
				j++
			}
			if j < len(s) && s[j] == 0x07 {
				j++
			} else if j < len(s) && s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
				j += 2
			}
			i = j
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}
