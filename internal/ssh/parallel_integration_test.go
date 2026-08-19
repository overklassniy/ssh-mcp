//go:build integration

package ssh

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/overklassniy/ssh-mcp/internal/config"
	"github.com/overklassniy/ssh-mcp/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// alpineTestConfig returns a config pointing at the Alpine-SSH-test WSL instance.
// The host and port are read from environment variables with defaults.
func alpineTestConfig() *config.Config {
	host := os.Getenv("ALPINE_SSH_HOST")
	if host == "" {
		host = "172.30.153.167"
	}
	port := 22
	if p := os.Getenv("ALPINE_SSH_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}
	user := os.Getenv("ALPINE_SSH_USER")
	if user == "" {
		user = "testuser"
	}
	pass := os.Getenv("ALPINE_SSH_PASS")
	if pass == "" {
		pass = "testpass"
	}

	cfg, _ := config.FromServer(config.ServerConfig{
		Name:               "alpine",
		Host:               host,
		Port:               port,
		Username:           user,
		Password:           pass,
		Transport:          "exec",
		AllowedLocalPaths:  []string{"/tmp", os.TempDir()},
		AllowedRemotePaths: []string{"/tmp", "/home/testuser"},
	})
	return cfg
}

// TestParallelExecCommands runs multiple commands in parallel against the
// real Alpine SSH server. Use with -race to detect data races.
func TestParallelExecCommands(t *testing.T) {
	cfg := alpineTestConfig()
	pol, _ := policy.New(nil, nil)
	client := NewClient(&cfg.Servers[0], pol)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	const numGoroutines = 20
	const commandsPerGoroutine = 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*commandsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < commandsPerGoroutine; j++ {
				cmd := fmt.Sprintf("echo goroutine-%d-cmd-%d", id, j)
				result, err := ExecCommand(ctx, client, cmd, "", 10*time.Second)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d cmd %d: %w", id, j, err)
					return
				}
				expected := fmt.Sprintf("goroutine-%d-cmd-%d", id, j)
				if result.ExitCode != 0 {
					errors <- fmt.Errorf("goroutine %d cmd %d: exit code %d", id, j, result.ExitCode)
					return
				}
				if !contains(result.Stdout, expected) {
					errors <- fmt.Errorf("goroutine %d cmd %d: expected %q in stdout, got %q", id, j, expected, result.Stdout)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// TestParallelConnectionManagerDedup verifies that concurrent GetClient
// calls for the same server name are deduplicated into a single connection.
func TestParallelConnectionManagerDedup(t *testing.T) {
	cfg := alpineTestConfig()
	manager := NewConnectionManager(cfg)
	defer manager.Disconnect()

	ctx := context.Background()
	const numGoroutines = 20

	var wg sync.WaitGroup
	clients := make([]SSHClient, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			client, err := manager.GetClient(ctx, "alpine")
			if err != nil {
				errors <- err
				return
			}
			clients[idx] = client
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// All goroutines should have gotten the same client instance
	firstClient := clients[0]
	require.NotNil(t, firstClient)
	for i := 1; i < numGoroutines; i++ {
		assert.Same(t, firstClient, clients[i], "goroutine %d got a different client", i)
	}
}

// TestParallelSFTPTransfers uploads and downloads multiple files concurrently.
func TestParallelSFTPTransfers(t *testing.T) {
	cfg := alpineTestConfig()
	pol, _ := policy.New(nil, nil)
	client := NewClient(&cfg.Servers[0], pol)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	sftpClient, err := NewSFTPClient(client)
	require.NoError(t, err)
	defer sftpClient.Close()

	// Create local test files
	tmpDir := t.TempDir()
	const numFiles = 10

	for i := 0; i < numFiles; i++ {
		content := fmt.Sprintf("content for file %d", i)
		path := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
		os.WriteFile(path, []byte(content), 0644)
	}

	// Upload all files in parallel
	var wg sync.WaitGroup
	errors := make(chan error, numFiles)

	for i := 0; i < numFiles; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			localPath := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", idx))
			remotePath := fmt.Sprintf("/tmp/parallel_test_%d.txt", idx)
			err := sftpClient.Upload(ctx, localPath, remotePath)
			if err != nil {
				errors <- fmt.Errorf("upload %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Download all files in parallel to different local paths
	downloadDir := filepath.Join(tmpDir, "downloaded")
	os.MkdirAll(downloadDir, 0755)

	var wg2 sync.WaitGroup
	errors2 := make(chan error, numFiles)

	for i := 0; i < numFiles; i++ {
		wg2.Add(1)
		go func(idx int) {
			defer wg2.Done()
			remotePath := fmt.Sprintf("/tmp/parallel_test_%d.txt", idx)
			localPath := filepath.Join(downloadDir, fmt.Sprintf("downloaded_%d.txt", idx))
			err := sftpClient.Download(ctx, remotePath, localPath)
			if err != nil {
				errors2 <- fmt.Errorf("download %d: %w", idx, err)
				return
			}
			// Verify content
			data, err := os.ReadFile(localPath)
			if err != nil {
				errors2 <- fmt.Errorf("read %d: %w", idx, err)
				return
			}
			expected := fmt.Sprintf("content for file %d", idx)
			if string(data) != expected {
				errors2 <- fmt.Errorf("content mismatch %d: expected %q, got %q", idx, expected, string(data))
			}
		}(i)
	}

	wg2.Wait()
	close(errors2)

	for err := range errors2 {
		t.Error(err)
	}
}

// TestParallelExecAndSFTP mixes command execution and SFTP transfers
// concurrently to test for resource contention.
func TestParallelExecAndSFTP(t *testing.T) {
	cfg := alpineTestConfig()
	pol, _ := policy.New(nil, nil)
	client := NewClient(&cfg.Servers[0], pol)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*2)

	// Half the goroutines run commands
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cmd := fmt.Sprintf("echo mixed-exec-%d", id)
			result, err := ExecCommand(ctx, client, cmd, "", 10*time.Second)
			if err != nil {
				errors <- fmt.Errorf("exec %d: %w", id, err)
				return
			}
			if result.ExitCode != 0 {
				errors <- fmt.Errorf("exec %d: exit code %d", id, result.ExitCode)
			}
		}(i)
	}

	// Half the goroutines do SFTP list-remote
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sftpClient, err := NewSFTPClient(client)
			if err != nil {
				errors <- fmt.Errorf("sftp init %d: %w", id, err)
				return
			}
			defer sftpClient.Close()
			_, err = sftpClient.ReadDir(ctx, "/tmp")
			if err != nil {
				errors <- fmt.Errorf("sftp readdir %d: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// TestConcurrentConnectClose tests that connecting and closing concurrently
// does not cause panics or data races.
func TestConcurrentConnectClose(t *testing.T) {
	cfg := alpineTestConfig()

	const numIterations = 5
	for i := 0; i < numIterations; i++ {
		pol, _ := policy.New(nil, nil)
		client := NewClient(&cfg.Servers[0], pol)

		ctx := context.Background()
		err := client.Connect(ctx)
		require.NoError(t, err)

		// Start a goroutine that runs commands
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = ExecCommand(ctx, client, "echo concurrent", "", 5*time.Second)
		}()

		// Close while command might be running
		time.Sleep(10 * time.Millisecond)
		client.Close()
		wg.Wait()
	}
}

// TestConnectionReconnectAfterClose verifies that the connection manager
// can reconnect after a connection is closed.
func TestConnectionReconnectAfterClose(t *testing.T) {
	cfg := alpineTestConfig()
	manager := NewConnectionManager(cfg)
	defer manager.Disconnect()

	ctx := context.Background()

	// First connection
	client1, err := manager.GetClient(ctx, "alpine")
	require.NoError(t, err)
	require.True(t, client1.IsConnected())

	// Invalidate the connection
	manager.Invalidate("alpine")
	require.False(t, client1.IsConnected())

	// Should reconnect
	client2, err := manager.GetClient(ctx, "alpine")
	require.NoError(t, err)
	require.True(t, client2.IsConnected())
	assert.NotSame(t, client1, client2)

	// Run a command on the reconnected client
	result, err := ExecCommand(ctx, client2, "echo reconnected", "", 10*time.Second)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "reconnected")
}

// TestParallelPortForwards opens multiple local port forwards concurrently.
func TestParallelPortForwards(t *testing.T) {
	cfg := alpineTestConfig()
	pol, _ := policy.New(nil, nil)
	client := NewClient(&cfg.Servers[0], pol)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	fm := NewForwardManager()

	const numForwards = 5
	var wg sync.WaitGroup
	errors := make(chan error, numForwards)
	entries := make([]*ForwardEntry, numForwards)

	for i := 0; i < numForwards; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			localAddr := fmt.Sprintf("127.0.0.1:%d", 19000+idx)
			entry, err := fm.OpenLocalForward(ctx, client, localAddr, "localhost:22")
			if err != nil {
				errors <- fmt.Errorf("forward %d: %w", idx, err)
				return
			}
			entries[idx] = entry
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Verify all forwards are listed
	list := fm.ListForwards()
	assert.Equal(t, numForwards, len(list), "expected %d forwards, got %d", numForwards, len(list))

	// Close all forwards
	fm.CloseAll()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, len(fm.ListForwards()))
}

// TestKeepaliveConcurrentWithExec verifies that keepalive goroutine
// does not race with command execution.
func TestKeepaliveConcurrentWithExec(t *testing.T) {
	cfg := alpineTestConfig()
	// Set a short keepalive interval to increase race likelihood
	cfg.Servers[0].KeepaliveInterval = config.Duration{Duration: 100 * time.Millisecond}

	pol, _ := policy.New(nil, nil)
	client := NewClient(&cfg.Servers[0], pol)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Close()

	// Run commands while keepalive is active
	const numCommands = 20
	var wg sync.WaitGroup
	errors := make(chan error, numCommands)

	for i := 0; i < numCommands; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := ExecCommand(ctx, client, fmt.Sprintf("echo keepalive-test-%d", id), "", 5*time.Second)
			if err != nil {
				errors <- fmt.Errorf("cmd %d: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsString(s, substr))
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure net import is used
var _ = net.JoinHostPort
