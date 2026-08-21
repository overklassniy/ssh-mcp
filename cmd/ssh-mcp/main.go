// Package main is the entry point for the ssh-mcp binary.
// It parses CLI flags, loads configuration, and starts the MCP stdio server.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/overklassniy/ssh-mcp/internal/config"
	"github.com/overklassniy/ssh-mcp/internal/mcp"
	"github.com/overklassniy/ssh-mcp/internal/sshconfig"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "ssh-mcp",
		Short:   "SSH MCP server - execute commands and transfer files on remote servers via MCP",
		Long:    "ssh-mcp is a Model Context Protocol server that provides SSH command execution, SFTP file transfers, and port forwarding for MCP-compatible AI assistants.",
		Version: version,
		RunE:    run,
	}

	// Config file
	rootCmd.Flags().String("config", "", "Path to TOML config file (ssh-mcp.toml)")

	// Single-server mode flags
	rootCmd.Flags().String("host", "", "SSH host (single-server mode)")
	rootCmd.Flags().Int("port", 22, "SSH port")
	rootCmd.Flags().StringP("user", "u", "", "SSH username")
	rootCmd.Flags().StringP("password", "p", "", "SSH password")
	rootCmd.Flags().String("private-key", "", "Path to private key file")
	rootCmd.Flags().String("passphrase", "", "Passphrase for private key (also from $SSH_MCP_PASSPHRASE)")
	rootCmd.Flags().String("agent", "", "SSH agent: 'env' (uses $SSH_AUTH_SOCK) or explicit socket path")

	// Proxy
	rootCmd.Flags().String("proxy", "", "Proxy URL (socks5://, http://, or https://)")

	// Transport
	rootCmd.Flags().String("transport", "exec", "Transport mode: 'exec' or 'shell'")
	rootCmd.Flags().String("shell-ready-timeout", "10s", "Shell ready timeout (duration string)")

	// Security
	rootCmd.Flags().StringSlice("whitelist", nil, "Command whitelist regex patterns (repeatable)")
	rootCmd.Flags().StringSlice("blacklist", nil, "Command blacklist regex patterns (repeatable)")
	rootCmd.Flags().StringSlice("allowed-local-paths", nil, "Allowed local path roots (repeatable)")
	rootCmd.Flags().StringSlice("allowed-remote-paths", nil, "Allowed remote path roots (repeatable)")

	// Command template
	rootCmd.Flags().String("command-template", "", "Command template with <command> or <quotedCommand> placeholder")

	// PTY
	rootCmd.Flags().Bool("pty", true, "Allocate a PTY for command execution")
	rootCmd.Flags().Bool("no-pty", false, "Disable PTY allocation (overrides --pty)")

	// 2FA
	rootCmd.Flags().Bool("try-keyboard", false, "Enable keyboard-interactive auth (for 2FA/MFA)")

	// Pre-connect
	rootCmd.Flags().Bool("pre-connect", false, "Connect to all servers on startup")

	// SSH config
	rootCmd.Flags().String("ssh-config", "", "Path to SSH config file (default: ~/.ssh/config)")

	// Logging
	rootCmd.Flags().String("log-level", "info", "Log level: debug, info, or error")

	// Register the install and snippet subcommands. They are defined in
	// install.go so the root command stays focused on the default run
	// mode.
	rootCmd.AddCommand(newInstallCmd())
	rootCmd.AddCommand(newSnippetCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Set up logging to stderr (never stdout, which is for MCP stdio)
	logLevel := cmd.Flag("log-level").Value.String()
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Load configuration
	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := loadConfig(cmd)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Create and run the MCP server.
	// configPath is passed so the server can hot-reload the config file
	// when it changes. In single-server CLI mode configPath is empty and
	// no watcher is started.
	srv := mcp.New(cfg, configPath)
	return srv.Run()
}

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	configPath, _ := cmd.Flags().GetString("config")

	if configPath != "" {
		return config.Load(configPath)
	}

	// Single-server mode from CLI flags
	host, _ := cmd.Flags().GetString("host")
	if host == "" {
		return nil, fmt.Errorf("either --config or --host must be provided")
	}

	port, _ := cmd.Flags().GetInt("port")
	user, _ := cmd.Flags().GetString("user")
	password, _ := cmd.Flags().GetString("password")
	if password == "" {
		// Allow MCPB and other launchers to inject the password via env
		// instead of a CLI flag, which is cleaner for sensitive values.
		password = os.Getenv("SSH_MCP_PASSWORD")
	}
	privateKey, _ := cmd.Flags().GetString("private-key")
	passphrase, _ := cmd.Flags().GetString("passphrase")
	if passphrase == "" {
		passphrase = os.Getenv("SSH_MCP_PASSPHRASE")
	}
	agentSpec, _ := cmd.Flags().GetString("agent")
	proxy, _ := cmd.Flags().GetString("proxy")
	transport, _ := cmd.Flags().GetString("transport")
	shellReadyTimeout, _ := cmd.Flags().GetString("shell-ready-timeout")
	whitelist, _ := cmd.Flags().GetStringSlice("whitelist")
	blacklist, _ := cmd.Flags().GetStringSlice("blacklist")
	allowedLocal, _ := cmd.Flags().GetStringSlice("allowed-local-paths")
	allowedRemote, _ := cmd.Flags().GetStringSlice("allowed-remote-paths")
	commandTemplate, _ := cmd.Flags().GetString("command-template")
	tryKeyboard, _ := cmd.Flags().GetBool("try-keyboard")
	preConnect, _ := cmd.Flags().GetBool("pre-connect")
	sshConfigPath, _ := cmd.Flags().GetString("ssh-config")

	// PTY handling
	pty := true
	if noPty, _ := cmd.Flags().GetBool("no-pty"); noPty {
		pty = false
	} else if ptyFlag, _ := cmd.Flags().GetBool("pty"); !ptyFlag {
		pty = false
	}

	// Resolve SSH config alias if host looks like an alias
	if !strings.Contains(host, ".") && !isIPAddress(host) && sshConfigPath != "" {
		entry, err := sshconfig.Lookup(host, sshConfigPath)
		if err == nil && entry != nil {
			if entry.HostName != "" {
				host = entry.HostName
			}
			if user == "" {
				user = entry.User
			}
			if port == 22 && entry.Port != 0 {
				port = entry.Port
			}
			if privateKey == "" && entry.IdentityFile != "" {
				privateKey = entry.IdentityFile
			}
		}
	}

	srv := config.ServerConfig{
		Name:              "default",
		Host:              host,
		Port:              port,
		Username:          user,
		Password:          password,
		PrivateKey:        privateKey,
		Passphrase:        passphrase,
		Agent:             agentSpec,
		Proxy:             proxy,
		Transport:         transport,
		ShellReadyTimeout: config.Duration{},
		TryKeyboard:       tryKeyboard,
		PreConnect:        preConnect,
		Whitelist:         whitelist,
		Blacklist:         blacklist,
		AllowedLocalPaths:  allowedLocal,
		AllowedRemotePaths: allowedRemote,
		CommandTemplate:   commandTemplate,
		PTY:               &pty,
	}

	// Parse shell-ready-timeout
	if shellReadyTimeout != "" {
		if err := srv.ShellReadyTimeout.UnmarshalText([]byte(shellReadyTimeout)); err != nil {
			return nil, fmt.Errorf("invalid shell-ready-timeout: %w", err)
		}
	}

	return config.FromServer(srv)
}

// isIPAddress checks whether s looks like an IP address.
func isIPAddress(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 3 {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}
