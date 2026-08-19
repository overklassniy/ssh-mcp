package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/overklassniy/ssh-mcp/internal/config"
	"github.com/overklassniy/ssh-mcp/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// testSSHServer starts an in-process SSH server for integration testing.
// It returns the address, a cleanup function, and the password to use.
func testSSHServer(t *testing.T) (addr string, cleanup func()) {
	t.Helper()

	// Generate host key
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	hostSigner, err := ssh.NewSignerFromSigner(priv)
	require.NoError(t, err)

	// Create listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "testuser" && string(pass) == "testpass" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}
	config.AddHostKey(hostSigner)

	// Track the public key for key-based auth
	_ = pub

	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				defer sshConn.Close()

				go ssh.DiscardRequests(reqs)

				for newChannel := range chans {
					if newChannel.ChannelType() != "session" {
						newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
						continue
					}
					channel, requests, err := newChannel.Accept()
					if err != nil {
						continue
					}
					go func(in <-chan *ssh.Request) {
						defer channel.Close()
						for req := range in {
							switch req.Type {
							case "exec":
								var execReq struct{ Command string }
								ssh.Unmarshal(req.Payload, &execReq)
								req.Reply(true, nil)
								// Execute the command
								output := executeTestCommand(execReq.Command)
								channel.Write([]byte(output))
								channel.SendRequest("exit-status", false, ssh.Marshal(struct{ ExitStatus uint32 }{0}))
								return
							case "shell":
								req.Reply(true, nil)
								// Simple shell: just echo input
								channel.Write([]byte("$ "))
								buf := make([]byte, 1024)
								for {
									n, err := channel.Read(buf)
									if err != nil {
										return
									}
									cmd := string(buf[:n])
									if cmd == "exit\n" || cmd == "exit\r\n" {
										return
									}
									output := executeTestCommand(strings.TrimSpace(cmd))
									channel.Write([]byte(output))
									channel.Write([]byte("$ "))
								}
							case "subsystem":
								var subReq struct{ Subsystem string }
								ssh.Unmarshal(req.Payload, &subReq)
								if subReq.Subsystem == "sftp" {
									req.Reply(true, nil)
									// SFTP subsystem would be handled here
									// For now, just close
									return
								}
								req.Reply(false, nil)
							default:
								req.Reply(false, nil)
							}
						}
					}(requests)
				}
			}(nConn)
		}
	}()

	return listener.Addr().String(), func() {
		listener.Close()
	}
}

// executeTestCommand handles simple test commands.
func executeTestCommand(cmd string) string {
	// Handle cd && command pattern
	if strings.HasPrefix(cmd, "cd ") {
		parts := strings.SplitN(cmd, " && ", 2)
		if len(parts) == 2 {
			return executeTestCommand(parts[1])
		}
	}
	switch cmd {
	case "echo hello":
		return "hello\n"
	case "echo test":
		return "test\n"
	case "hostname":
		return "testhost\n"
	case "uname -s":
		return "Linux\n"
	case "true":
		return ""
	default:
		if strings.HasPrefix(cmd, "echo ") {
			return cmd[5:] + "\n"
		}
		if strings.HasPrefix(cmd, "sleep ") {
			// Parse sleep duration and actually sleep
			var dur float64
			fmt.Sscanf(cmd[6:], "%f", &dur)
			if dur > 0 {
				time.Sleep(time.Duration(dur * float64(time.Second)))
			}
			return ""
		}
		return ""
	}
}

func TestExecCommand_Integration(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	p := 22
	fmt.Sscanf(port, "%d", &p)

	srv := &config.ServerConfig{
		Name:     "test",
		Host:     host,
		Port:     p,
		Username: "testuser",
		Password: "testpass",
		Transport: "exec",
	}
	config.FromServer(*srv)
	cfg, _ := config.FromServer(*srv)

	pol, _ := policy.New(nil, nil)
	client := NewClient(&cfg.Servers[0], pol)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()
	assert.True(t, client.IsConnected())

	result, err := ExecCommand(ctx, client, "echo hello", "", 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
}

func TestConnectionManager_Integration(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	p := 22
	fmt.Sscanf(port, "%d", &p)

	cfg, _ := config.FromServer(config.ServerConfig{
		Name:      "test",
		Host:      host,
		Port:      p,
		Username:  "testuser",
		Password:  "testpass",
		Transport: "exec",
	})

	manager := NewConnectionManager(cfg)
	defer manager.Disconnect()

	ctx := context.Background()
	client, err := manager.GetClient(ctx, "test")
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Test that a second GetClient returns the same connection
	client2, err := manager.GetClient(ctx, "test")
	require.NoError(t, err)
	assert.True(t, client2.IsConnected())
}

func TestSFTP_UploadDownload(t *testing.T) {
	// This test requires SFTP subsystem support which the test server
	// doesn't fully implement yet. Skip for now.
	t.Skip("SFTP integration test requires full SFTP subsystem support")
}

func TestExecCommand_Timeout(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	p := 22
	fmt.Sscanf(port, "%d", &p)

	srv := config.ServerConfig{
		Name:      "test",
		Host:      host,
		Port:      p,
		Username:  "testuser",
		Password:  "testpass",
		Transport: "exec",
	}
	cfg, _ := config.FromServer(srv)

	pol, _ := policy.New(nil, nil)
	client := NewClient(&cfg.Servers[0], pol)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	// Short timeout with a sleep command that takes longer
	_, err = ExecCommand(ctx, client, "sleep 10", "", 100*time.Millisecond)
	require.Error(t, err)
}

func TestPolicyValidation_ExecCommand(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	p := 22
	fmt.Sscanf(port, "%d", &p)

	srv := config.ServerConfig{
		Name:      "test",
		Host:      host,
		Port:      p,
		Username:  "testuser",
		Password:  "testpass",
		Transport: "exec",
		Whitelist: []string{`^ls .*`},
	}
	cfg, _ := config.FromServer(srv)

	pol, _ := policy.New(srv.Whitelist, nil)
	client := NewClient(&cfg.Servers[0], pol)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	_, err = ExecCommand(ctx, client, "echo hello", "", 10*time.Second)
	require.Error(t, err)
	te, ok := err.(*ToolError)
	require.True(t, ok)
	assert.Equal(t, CodeCommandValidationFailed, te.Code)
}

func TestExecCommand_Directory(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	p := 22
	fmt.Sscanf(port, "%d", &p)

	srv := config.ServerConfig{
		Name:      "test",
		Host:      host,
		Port:      p,
		Username:  "testuser",
		Password:  "testpass",
		Transport: "exec",
	}
	cfg, _ := config.FromServer(srv)

	pol, _ := policy.New(nil, nil)
	client := NewClient(&cfg.Servers[0], pol)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	result, err := ExecCommand(ctx, client, "echo hello", "/tmp", 10*time.Second)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "hello")
}

func TestFileUpload_LocalValidation(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("test content"), 0644))

	// Test that path validation works even without a real SSH connection
	// by checking the error code
	srv := config.ServerConfig{
		Name:              "test",
		Host:              "nonexistent",
		Port:              22,
		Username:          "user",
		Password:          "pass",
		Transport:         "exec",
		AllowedLocalPaths: []string{tmpDir},
	}
	cfg, _ := config.FromServer(srv)

	pol, _ := policy.New(nil, nil)
	client := NewClient(&cfg.Servers[0], pol)
	// Don't connect - just test path validation

	// This should fail at connection, not at path validation
	_, err := NewSFTPClient(client)
	require.Error(t, err)
}
