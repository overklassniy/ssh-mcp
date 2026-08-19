package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/overklassniy/ssh-mcp/internal/config"
	"github.com/overklassniy/ssh-mcp/internal/policy"
)

// ConnectionManager manages SSH connections for multiple named servers.
// It supports lazy connection, reconnection with backoff, and graceful shutdown.
type ConnectionManager struct {
	mu       sync.RWMutex
	clients  map[string]SSHClient
	config   *config.Config
	policies map[string]*policy.Policy
	pending  map[string]*pendingConnect
	closed   bool
}

type pendingConnect struct {
	done chan struct{}
	err  error
}

// NewConnectionManager creates a manager from the given configuration.
// Connections are not established until Connect or ConnectAll is called.
func NewConnectionManager(cfg *config.Config) *ConnectionManager {
	m := &ConnectionManager{
		clients:  make(map[string]SSHClient),
		config:   cfg,
		policies: make(map[string]*policy.Policy),
		pending:  make(map[string]*pendingConnect),
	}
	for i := range cfg.Servers {
		srv := &cfg.Servers[i]
		pol, err := policy.New(srv.Whitelist, srv.Blacklist)
		if err != nil {
			slog.Error("failed to compile policy", "server", srv.Name, "error", err)
			continue
		}
		m.policies[srv.Name] = pol
		if !pol.HasWhitelist() {
			slog.Warn("no command whitelist configured; all commands will be allowed",
				"server", srv.Name)
		}
		if len(srv.AllowedRemotePaths) == 0 {
			slog.Warn("no allowed_remote_paths configured; all absolute remote paths will be allowed",
				"server", srv.Name)
		}
	}
	return m
}

// GetClient returns the SSHClient for the named server, connecting lazily
// if needed. If name is empty, the default (first) server is used.
func (m *ConnectionManager) GetClient(ctx context.Context, name string) (SSHClient, error) {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, NewToolError(CodeSSHConnectionFailed, "manager is shut down", false)
	}
	if name == "" {
		name = m.config.DefaultServerName()
	}
	if name == "" {
		m.mu.RUnlock()
		return nil, NewToolError(CodeSSHConnectionFailed, "no servers configured", false)
	}
	client, ok := m.clients[name]
	m.mu.RUnlock()

	if ok && client.IsConnected() {
		return client, nil
	}

	// Need to connect — acquire write lock for dedup
	return m.connectDedup(ctx, name)
}

// connectDedup connects to the named server, deduplicating concurrent
// connection attempts for the same server.
func (m *ConnectionManager) connectDedup(ctx context.Context, name string) (SSHClient, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, NewToolError(CodeSSHConnectionFailed, "manager is shut down", false)
	}

	// Check if already connected by another goroutine
	if client, ok := m.clients[name]; ok && client.IsConnected() {
		m.mu.Unlock()
		return client, nil
	}

	// Check for pending connection
	if p, ok := m.pending[name]; ok {
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.done:
		}
		if p.err != nil {
			return nil, p.err
		}
		m.mu.RLock()
		client := m.clients[name]
		m.mu.RUnlock()
		return client, nil
	}

	// Start a new connection attempt
	p := &pendingConnect{done: make(chan struct{})}
	m.pending[name] = p
	m.mu.Unlock()

	// Build the client and connect
	srv := m.config.GetServer(name)
	if srv == nil {
		p.err = NewToolError(CodeSSHConnectionFailed, fmt.Sprintf("server %q not found", name), false)
		close(p.done)
		m.mu.Lock()
		delete(m.pending, name)
		m.mu.Unlock()
		return nil, p.err
	}

	pol := m.policies[name]
	client := NewClient(srv, pol)
	err := client.Connect(ctx)

	m.mu.Lock()
	delete(m.pending, name)
	if err == nil {
		m.clients[name] = client
	} else {
		// Retry with backoff
		err = m.reconnectWithBackoff(ctx, name, client, err)
		if err == nil {
			m.clients[name] = client
		}
	}
	m.mu.Unlock()

	close(p.done)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// reconnectWithBackoff retries a failed connection with exponential backoff.
func (m *ConnectionManager) reconnectWithBackoff(ctx context.Context, name string, client SSHClient, initialErr error) error {
	maxRetries := 3
	baseDelay := 1 * time.Second
	maxDelay := 10 * time.Second

	err := initialErr
	for attempt := 1; attempt <= maxRetries; attempt++ {
		delay := baseDelay * time.Duration(1<<(attempt-1))
		if delay > maxDelay {
			delay = maxDelay
		}
		slog.Warn("connection attempt failed, retrying",
			"server", name, "attempt", attempt, "delay", delay, "error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		err = client.Connect(ctx)
		if err == nil {
			return nil
		}
	}
	return err
}

// ConnectAll connects to all configured servers in parallel.
// Errors for individual servers are logged but do not stop other connections.
func (m *ConnectionManager) ConnectAll(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(m.config.Servers))

	for i := range m.config.Servers {
		name := m.config.Servers[i].Name
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.GetClient(ctx, name)
			if err != nil {
				slog.Error("failed to connect to server", "server", name, "error", err)
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	var firstErr error
	for err := range errCh {
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Disconnect closes all connections and marks the manager as closed.
func (m *ConnectionManager) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for name, client := range m.clients {
		if err := client.Close(); err != nil {
			slog.Warn("error closing connection", "server", name, "error", err)
		}
	}
	m.clients = make(map[string]SSHClient)
}

// Invalidate closes the connection for the named server, forcing a reconnect
// on the next GetClient call.
func (m *ConnectionManager) Invalidate(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if client, ok := m.clients[name]; ok {
		_ = client.Close()
		delete(m.clients, name)
	}
}

// ListServers returns the names of all configured servers.
func (m *ConnectionManager) ListServers() []string {
	return m.config.ServerNames()
}

// Config returns the underlying configuration.
func (m *ConnectionManager) Config() *config.Config {
	return m.config
}

// Policy returns the policy for the named server.
func (m *ConnectionManager) Policy(name string) *policy.Policy {
	if name == "" {
		name = m.config.DefaultServerName()
	}
	return m.policies[name]
}
