// Package mcp wires the SSH connection manager to the MCP protocol server
// using mark3labs/mcp-go with stdio transport.
package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/overklassniy/ssh-mcp/internal/config"
	"github.com/overklassniy/ssh-mcp/internal/mcp/tools"
	"github.com/overklassniy/ssh-mcp/internal/ssh"
)

// Server wraps the mcp-go MCP server with the SSH connection manager.
type Server struct {
	mcpSrv  *server.MCPServer
	manager *ssh.ConnectionManager
}

// New creates a new MCP server with all tools registered.
func New(cfg *config.Config) *Server {
	mcpSrv := server.NewMCPServer("ssh-mcp", "1.0.0",
		server.WithToolCapabilities(true),
	)

	manager := ssh.NewConnectionManager(cfg)

	s := &Server{
		mcpSrv:  mcpSrv,
		manager: manager,
	}

	tools.RegisterAll(mcpSrv, manager)

	return s
}

// Run starts the MCP server on stdio and blocks until the server stops
// or a shutdown signal is received.
func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Info("received shutdown signal, disconnecting", "signal", sig)
		s.manager.Disconnect()
		cancel()
	}()

	// Pre-connect if any server has pre_connect=true
	if s.manager.Config().PreConnect() {
		slog.Info("pre-connecting to all configured servers")
		if err := s.manager.ConnectAll(ctx); err != nil {
			slog.Warn("some pre-connections failed", "error", err)
		}
	}

	slog.Info("starting MCP server on stdio")
	if err := server.ServeStdio(s.mcpSrv); err != nil {
		return fmt.Errorf("MCP stdio server: %w", err)
	}

	s.manager.Disconnect()
	return nil
}

// Manager returns the connection manager (for testing).
func (s *Server) Manager() *ssh.ConnectionManager {
	return s.manager
}

// Ensure the mcp package is used (for the import).
var _ = mcp.NewTool
