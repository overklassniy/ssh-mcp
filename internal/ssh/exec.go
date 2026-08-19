package ssh

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// ExecCommand runs a command on the remote server in exec mode.
// It handles PTY allocation, output capping, timeout, and exit code extraction.
// The command is validated against the server's policy before execution.
func ExecCommand(ctx context.Context, client SSHClient, command string, directory string, timeout time.Duration) (*ExecResult, error) {
	srv := client.Config()

	// Apply command template if configured
	fullCmd := applyCommandTemplate(srv.CommandTemplate, command, directory)

	// Validate against policy
	if pol := client.Policy(); pol != nil {
		allowed, reason := pol.Validate(fullCmd)
		if !allowed {
			return nil, NewToolError(CodeCommandValidationFailed, reason, false)
		}
	}

	return execCommandRaw(ctx, client, fullCmd, directory, timeout)
}

// ExecCommandBypass runs a command without policy validation.
// This is used by the status collector, which validates each probe
// command individually before batching them into a single script.
// The combined script would not match any whitelist pattern, so
// it must bypass the policy check.
func ExecCommandBypass(ctx context.Context, client SSHClient, command string, directory string, timeout time.Duration) (*ExecResult, error) {
	srv := client.Config()
	fullCmd := applyCommandTemplate(srv.CommandTemplate, command, directory)
	return execCommandRaw(ctx, client, fullCmd, directory, timeout)
}

// execCommandRaw runs the already-validated command on the remote server.
func execCommandRaw(ctx context.Context, client SSHClient, fullCmd, directory string, timeout time.Duration) (*ExecResult, error) {
	srv := client.Config()

	// Determine timeout
	if timeout <= 0 {
		timeout = srv.GetCommandTimeout()
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	session, err := client.NewSession()
	if err != nil {
		return nil, AsToolError(err, CodeSSHConnectionFailed)
	}
	defer session.Close()

	// Allocate PTY if configured
	if srv.GetPTY() {
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}
		if err := session.RequestPty("xterm", 80, 200, modes); err != nil {
			return nil, NewToolError(CodeCommandExecutionError,
				fmt.Sprintf("request PTY: %v", err), false)
		}
	}

	// Set working directory if specified
	if directory != "" {
		fullCmd = fmt.Sprintf("cd %s && %s", shellQuote(directory), fullCmd)
	}

	// Capture stdout and stderr separately
	var stdoutBuf, stderrBuf bytes.Buffer
	maxOutput := srv.GetMaxOutputBytes()

	stdoutLimited := &limitedWriter{w: &stdoutBuf, max: maxOutput}
	stderrLimited := &limitedWriter{w: &stderrBuf, max: maxOutput}

	session.Stdout = stdoutLimited
	session.Stderr = stderrLimited

	start := time.Now()

	// Run the command
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- session.Run(fullCmd)
	}()

	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return &ExecResult{
				Stdout:    stdoutBuf.String(),
				Stderr:    stderrBuf.String(),
				ExitCode:  -1,
				Duration:  time.Since(start).String(),
				Truncated: stdoutLimited.truncated || stderrLimited.truncated,
			}, NewToolError(CodeCommandTimeout, "command timed out", true)
		}
		return nil, ctx.Err()
	case err := <-doneCh:
		exitCode := 0
		var signal string
		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				exitCode = exitErr.ExitStatus()
			} else {
				exitCode = -1
			}
		}

		return &ExecResult{
			Stdout:    stdoutBuf.String(),
			Stderr:    stderrBuf.String(),
			ExitCode:  exitCode,
			Duration:  time.Since(start).String(),
			Truncated: stdoutLimited.truncated || stderrLimited.truncated,
			Signal:    signal,
		}, nil
	}
}

// applyCommandTemplate wraps the user command in the configured template.
func applyCommandTemplate(template, command, directory string) string {
	if template == "" {
		return command
	}
	quoted := shellQuote(command)
	result := strings.ReplaceAll(template, "<quotedCommand>", quoted)
	result = strings.ReplaceAll(result, "<command>", command)
	return result
}

// shellQuote single-quotes a string for safe shell usage.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// limitedWriter wraps a writer and stops writing after max bytes.
// It sets the truncated flag when the limit is reached.
type limitedWriter struct {
	w         *bytes.Buffer
	max       int64
	written   int64
	truncated bool
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.written >= lw.max {
		lw.truncated = true
		return len(p), nil
	}
	remaining := lw.max - lw.written
	if int64(len(p)) > remaining {
		lw.w.Write(p[:remaining])
		lw.written = lw.max
		lw.truncated = true
		return len(p), nil
	}
	n, err := lw.w.Write(p)
	lw.written += int64(n)
	return n, err
}
