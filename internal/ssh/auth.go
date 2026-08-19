package ssh

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/overklassniy/ssh-mcp/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// BuildAuthMethods constructs the list of SSH authentication methods from
// the server configuration. The order is: private key, agent, password,
// keyboard-interactive. Only configured methods are included.
// Returns an error if no method can be constructed.
func BuildAuthMethods(srv *config.ServerConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if srv.PrivateKey != "" {
		m, err := buildPublicKeyAuth(srv.PrivateKey, srv.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("private key auth: %w", err)
		}
		methods = append(methods, m)
	}

	if srv.Agent != "" {
		m, err := buildAgentAuth(srv.Agent)
		if err != nil {
			slog.Warn("agent auth unavailable, skipping", "error", err)
		} else {
			methods = append(methods, m)
		}
	}

	if srv.Password != "" {
		methods = append(methods, ssh.Password(srv.Password))
	}

	if srv.TryKeyboard {
		m := buildKeyboardInteractiveAuth(srv.Password)
		methods = append(methods, m)
	}

	if len(methods) == 0 {
		return nil, NewToolError(CodeSSHAuthMissing,
			"no authentication method could be constructed", false)
	}
	return methods, nil
}

// buildPublicKeyAuth reads a private key file and returns a PublicKeys auth method.
// If passphrase is non-empty, it uses ParsePrivateKeyWithPassphrase.
func buildPublicKeyAuth(keyPath, passphrase string) (ssh.AuthMethod, error) {
	keyPath = config.ExpandHome(keyPath)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key %s: %w", keyPath, err)
	}

	var signer ssh.Signer
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(data)
	}
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return ssh.PublicKeys(signer), nil
}

// buildAgentAuth connects to the SSH agent and returns a PublicKeysCallback
// auth method. agentSpec is "env" (uses $SSH_AUTH_SOCK) or an explicit socket path.
func buildAgentAuth(agentSpec string) (ssh.AuthMethod, error) {
	socketPath := agentSpec
	if agentSpec == "env" {
		socketPath = os.Getenv("SSH_AUTH_SOCK")
		if socketPath == "" {
			return nil, errors.New("SSH_AUTH_SOCK not set")
		}
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to agent socket %s: %w", socketPath, err)
	}

	ag := agent.NewClient(conn)
	return ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		signers, err := ag.Signers()
		if err != nil {
			return nil, fmt.Errorf("agent signers: %w", err)
		}
		return signers, nil
	}), nil
}

// buildKeyboardInteractiveAuth creates a keyboard-interactive auth method
// that responds to password prompts with the configured password and to
// 2FA/MFA prompts with the value from $SSH_MCP_2FA_CODE.
func buildKeyboardInteractiveAuth(password string) ssh.AuthMethod {
	return ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
		// Log the instruction for debugging (without exposing secrets).
		if instruction != "" {
			slog.Debug("keyboard-interactive challenge",
				"name", name, "instruction", instruction,
				"num_questions", len(questions))
		}

		answers := make([]string, len(questions))
		for i, q := range questions {
			lower := strings.ToLower(q)
			if is2FAPrompt(lower) {
				code := os.Getenv("SSH_MCP_2FA_CODE")
				if code == "" {
					return nil, fmt.Errorf("2FA code required but SSH_MCP_2FA_CODE not set")
				}
				answers[i] = code
			} else {
				if password == "" {
					return nil, fmt.Errorf("keyboard-interactive password prompt but no password configured")
				}
				answers[i] = password
			}
		}
		return answers, nil
	})
}

// is2FAPrompt heuristically detects whether a prompt is asking for a 2FA/MFA code.
func is2FAPrompt(prompt string) bool {
	keywords := []string{
		"2fa", "mfa", "otp", "one-time", "one time",
		"verification code", "verification token",
		"authenticator", "totp", "duo",
	}
	for _, kw := range keywords {
		if strings.Contains(prompt, kw) {
			return true
		}
	}
	return false
}

// redactReader wraps an io.Reader to prevent accidental logging of its contents.
type redactReader struct {
	r io.Reader
}

func (rr *redactReader) Read(p []byte) (int, error) {
	return rr.r.Read(p)
}
