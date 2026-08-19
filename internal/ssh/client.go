package ssh

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/overklassniy/ssh-mcp/internal/config"
	"github.com/overklassniy/ssh-mcp/internal/policy"
	"golang.org/x/crypto/ssh"
)

// SSHClient is the interface for SSH operations. The real implementation
// wraps *ssh.Client; tests use fakes.
type SSHClient interface {
	// Connect establishes the SSH connection.
	Connect(ctx context.Context) error
	// Close closes the underlying SSH connection.
	Close() error
	// IsConnected reports whether the connection is alive.
	IsConnected() bool
	// NewSession opens a new SSH session for command execution.
	NewSession() (*ssh.Session, error)
	// Client returns the underlying *ssh.Client (for SFTP, forwarding, etc.).
	Client() *ssh.Client
	// Config returns the server configuration.
	Config() *config.ServerConfig
	// Policy returns the command policy for this server.
	Policy() *policy.Policy
}

// realClient wraps an *ssh.Client with connection lifecycle management.
// All access to client is protected by mu to prevent data races between
// keepalive, Close, NewSession, IsConnected, and Client.
type realClient struct {
	mu     sync.RWMutex
	srv    *config.ServerConfig
	pol    *policy.Policy
	client *ssh.Client
}

// NewClient creates a new SSHClient for the given server configuration.
func NewClient(srv *config.ServerConfig, pol *policy.Policy) SSHClient {
	return &realClient{srv: srv, pol: pol}
}

func (c *realClient) Connect(ctx context.Context) error {
	authMethods, err := BuildAuthMethods(c.srv)
	if err != nil {
		return err
	}

	sshConfig := &ssh.ClientConfig{
		User:            c.srv.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         c.srv.GetConnectionTimeout(),
	}

	addr := net.JoinHostPort(c.srv.Host, strconv.Itoa(c.srv.Port))

	if c.srv.Proxy != "" {
		conn, err := dialProxy(ctx, c.srv.Proxy, addr)
		if err != nil {
			return NewToolError(CodeSSHConnectionFailed, err.Error(), true)
		}
		sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
		if err != nil {
			conn.Close()
			return NewToolError(CodeSSHConnectionFailed, err.Error(), true)
		}
		c.mu.Lock()
		c.client = ssh.NewClient(sshConn, chans, reqs)
		c.mu.Unlock()
	} else {
		dialer := &net.Dialer{Timeout: c.srv.GetConnectionTimeout()}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return NewToolError(CodeSSHConnectionTimeout, err.Error(), true)
		}
		sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
		if err != nil {
			conn.Close()
			return NewToolError(CodeSSHConnectionFailed, err.Error(), true)
		}
		c.mu.Lock()
		c.client = ssh.NewClient(sshConn, chans, reqs)
		c.mu.Unlock()
	}

	// Start keepalive goroutine
	go c.keepalive()

	return nil
}

func (c *realClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return nil
	}
	err := c.client.Close()
	c.client = nil
	return err
}

func (c *realClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client != nil
}

func (c *realClient) NewSession() (*ssh.Session, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.client == nil {
		return nil, NewToolError(CodeSSHConnectionFailed, "not connected", true)
	}
	return c.client.NewSession()
}

func (c *realClient) Client() *ssh.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

func (c *realClient) Config() *config.ServerConfig {
	return c.srv
}

func (c *realClient) Policy() *policy.Policy {
	return c.pol
}

// keepalive sends periodic global requests to keep the connection alive.
func (c *realClient) keepalive() {
	interval := c.srv.GetKeepaliveInterval()
	if interval <= 0 {
		return
	}
	maxFailures := c.srv.KeepaliveCountMax
	if maxFailures <= 0 {
		maxFailures = 3
	}

	failures := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.RLock()
		client := c.client
		c.mu.RUnlock()
		if client == nil {
			return
		}
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		if err != nil {
			failures++
			if failures >= maxFailures {
				_ = c.Close()
				return
			}
		} else {
			failures = 0
		}
	}
}

// ExecResult holds the result of a command execution.
type ExecResult struct {
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exitCode"`
	Duration     string `json:"duration"`
	Truncated    bool   `json:"truncated"`
	Signal       string `json:"signal,omitempty"`
}
