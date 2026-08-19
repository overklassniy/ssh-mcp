// Package main is the entry point for the ssh-mcp binary.
// This file implements the `install` subcommand, which registers ssh-mcp
// in a supported AI client's MCP config file.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/overklassniy/ssh-mcp/internal/install"
	"github.com/spf13/cobra"
)

// newInstallCmd builds the `install` subcommand. It reuses the root
// command's single-server and config flags so an install can be driven
// either by a TOML config file or by individual SSH flags.
func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Register ssh-mcp in an AI client's MCP config",
		Long: `Register ssh-mcp in a supported AI client's MCP config file.

Supported clients: claude-desktop, claude-code, cursor, Devin, vscode, cline.

The command resolves its own executable path and writes a server entry
under the client's root key ("mcpServers", or "servers" for VS Code),
merging non-destructively alongside any existing servers. A backup of the
existing config is created with a .bak suffix before writing.

Either --config (path to a TOML file) or single-server flags (--host,
--user, ...) must be provided, exactly like the default run mode.`,
		RunE: runInstall,
	}

	cmd.Flags().String("client", "", "Target client: claude-desktop, claude-code, cursor, Devin, vscode, or cline")
	cmd.Flags().String("name", "ssh-mcp", "Server alias under which ssh-mcp is registered")
	cmd.Flags().Bool("dry-run", false, "Print the config that would be written without modifying any files")

	// Docker runtime mode. When set, the install entry uses `docker run`
	// instead of the local executable path. The --client flag still
	// selects which agent config file is written.
	cmd.Flags().Bool("docker", false, "Install as a `docker run` entry instead of a direct executable")
	cmd.Flags().String("docker-image", install.DefaultDockerImage, "Container image for docker install mode")

	// Config file (single source of truth when provided).
	cmd.Flags().String("config", "", "Path to TOML config file (ssh-mcp.toml)")

	// Single-server mode flags mirrored from the root command.
	cmd.Flags().String("host", "", "SSH host (single-server mode)")
	cmd.Flags().Int("port", 22, "SSH port")
	cmd.Flags().StringP("user", "u", "", "SSH username")
	cmd.Flags().StringP("password", "p", "", "SSH password (or $SSH_MCP_PASSWORD)")
	cmd.Flags().String("private-key", "", "Path to private key file")
	cmd.Flags().String("passphrase", "", "Passphrase for private key (or $SSH_MCP_PASSPHRASE)")
	cmd.Flags().String("agent", "", "SSH agent: 'env' (uses $SSH_AUTH_SOCK) or explicit socket path")
	cmd.Flags().String("proxy", "", "Proxy URL (socks5://, http://, or https://)")
	cmd.Flags().String("transport", "exec", "Transport mode: 'exec' or 'shell'")
	cmd.Flags().Bool("try-keyboard", false, "Enable keyboard-interactive auth (for 2FA/MFA)")
	cmd.Flags().Bool("pre-connect", false, "Connect to all servers on startup")
	cmd.Flags().String("ssh-config", "", "Path to SSH config file (default: ~/.ssh/config)")

	_ = cmd.MarkFlagRequired("client")
	return cmd
}

// runInstall builds the server entry from the resolved executable path and
// the provided config/flags, then delegates to install.Install.
func runInstall(cmd *cobra.Command, args []string) error {
	clientStr, _ := cmd.Flags().GetString("client")
	client := install.Client(clientStr)

	// Validate the client against the supported list before resolving paths,
	// so an unknown name produces a clear error listing the options.
	valid := false
	for _, c := range install.SupportedClients() {
		if c == client {
			valid = true
			break
		}
	}
	if !valid {
		names := make([]string, 0, len(install.SupportedClients()))
		for _, c := range install.SupportedClients() {
			names = append(names, string(c))
		}
		return fmt.Errorf("unsupported client %q; supported: %s", clientStr, strings.Join(names, ", "))
	}

	name, _ := cmd.Flags().GetString("name")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	useDocker, _ := cmd.Flags().GetBool("docker")

	var entry install.Entry
	var err error
	if useDocker {
		entry, err = buildDockerEntry(cmd)
		if err != nil {
			return err
		}
	} else {
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable path: %w", err)
		}
		// Resolve symlinks so the recorded command points at the real binary.
		if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
			execPath = resolved
		}
		entry, err = buildInstallEntry(cmd, execPath)
		if err != nil {
			return err
		}
	}

	res, err := install.Install(install.InstallOptions{
		Client: client,
		Name:   name,
		Entry:  entry,
		DryRun: dryRun,
	})
	if err != nil {
		return err
	}

	rootKey, _ := install.RootKey(client)
	if dryRun {
		fmt.Fprintf(os.Stdout, "Dry run: would write to %s under %q\n", res.Path, rootKey)
		fmt.Fprintln(os.Stdout, string(res.Content))
		return nil
	}

	fmt.Fprintf(os.Stdout, "Installed %q into %s under %q\n", name, res.Path, rootKey)
	if res.BackupPath != "" {
		fmt.Fprintf(os.Stdout, "Backup of previous config: %s\n", res.BackupPath)
	}
	fmt.Fprintln(os.Stdout, "Restart the client for the change to take effect.")
	return nil
}

// buildInstallEntry constructs the install.Entry from either a TOML config
// path or single-server CLI flags. Sensitive values (password, passphrase)
// are placed in the env map rather than args so they are not visible in
// process argument listings.
func buildInstallEntry(cmd *cobra.Command, execPath string) (install.Entry, error) {
	configPath, _ := cmd.Flags().GetString("config")

	if configPath != "" {
		abs, err := filepath.Abs(configPath)
		if err != nil {
			return install.Entry{}, fmt.Errorf("resolve config path: %w", err)
		}
		return install.Entry{
			Command: execPath,
			Args:    []string{"--config", abs},
		}, nil
	}

	// Single-server mode: mirror the root command's flag handling.
	host, _ := cmd.Flags().GetString("host")
	if host == "" {
		return install.Entry{}, fmt.Errorf("either --config or --host must be provided")
	}
	port, _ := cmd.Flags().GetInt("port")
	user, _ := cmd.Flags().GetString("user")
	password, _ := cmd.Flags().GetString("password")
	if password == "" {
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
	tryKeyboard, _ := cmd.Flags().GetBool("try-keyboard")
	preConnect, _ := cmd.Flags().GetBool("pre-connect")
	sshConfigPath, _ := cmd.Flags().GetString("ssh-config")

	args := []string{
		"--host", host,
		"--port", fmt.Sprintf("%d", port),
		"--user", user,
		"--transport", transport,
	}
	if privateKey != "" {
		args = append(args, "--private-key", privateKey)
	}
	if agentSpec != "" {
		args = append(args, "--agent", agentSpec)
	}
	if proxy != "" {
		args = append(args, "--proxy", proxy)
	}
	if tryKeyboard {
		args = append(args, "--try-keyboard")
	}
	if preConnect {
		args = append(args, "--pre-connect")
	}
	if sshConfigPath != "" {
		args = append(args, "--ssh-config", sshConfigPath)
	}

	env := map[string]string{}
	if password != "" {
		env["SSH_MCP_PASSWORD"] = password
	}
	if passphrase != "" {
		env["SSH_MCP_PASSPHRASE"] = passphrase
	}

	return install.Entry{
		Command: execPath,
		Args:    args,
		Env:     env,
	}, nil
}

// buildDockerEntry constructs an install.Entry that runs ssh-mcp inside a
// container via `docker run -i --rm`. The host home directory is mounted
// at the same absolute path so that ~/ paths in the TOML config resolve
// identically inside the container. The SSH agent socket is forwarded when
// SSH_AUTH_SOCK is set on the host.
//
// Either --config (preferred, supports multiple servers) or single-server
// flags (--host, --user, ...) must be provided, exactly like the direct
// install mode. Sensitive values stay in the host environment and are
// forwarded via -e, never on the command line.
func buildDockerEntry(cmd *cobra.Command) (install.Entry, error) {
	image, _ := cmd.Flags().GetString("docker-image")

	home, _ := os.UserHomeDir()
	agentSocket := os.Getenv("SSH_AUTH_SOCK")

	opts := install.DockerEntryOptions{
		Image:       image,
		Home:        home,
		AgentSocket: agentSocket,
	}

	configPath, _ := cmd.Flags().GetString("config")
	if configPath != "" {
		abs, err := filepath.Abs(configPath)
		if err != nil {
			return install.Entry{}, fmt.Errorf("resolve config path: %w", err)
		}
		opts.ConfigPath = abs
	} else {
		// Single-server mode: forward the CLI flags as extra args after
		// the image name, mirroring buildInstallEntry but without
		// sensitive values (those go through env passthrough).
		host, _ := cmd.Flags().GetString("host")
		if host == "" {
			return install.Entry{}, fmt.Errorf("either --config or --host must be provided")
		}
		port, _ := cmd.Flags().GetInt("port")
		user, _ := cmd.Flags().GetString("user")
		privateKey, _ := cmd.Flags().GetString("private-key")
		agentSpec, _ := cmd.Flags().GetString("agent")
		proxy, _ := cmd.Flags().GetString("proxy")
		transport, _ := cmd.Flags().GetString("transport")
		tryKeyboard, _ := cmd.Flags().GetBool("try-keyboard")
		preConnect, _ := cmd.Flags().GetBool("pre-connect")
		sshConfigPath, _ := cmd.Flags().GetString("ssh-config")

		extra := []string{
			"--host", host,
			"--port", fmt.Sprintf("%d", port),
			"--user", user,
			"--transport", transport,
		}
		if privateKey != "" {
			extra = append(extra, "--private-key", privateKey)
		}
		if agentSpec != "" {
			extra = append(extra, "--agent", agentSpec)
		}
		if proxy != "" {
			extra = append(extra, "--proxy", proxy)
		}
		if tryKeyboard {
			extra = append(extra, "--try-keyboard")
		}
		if preConnect {
			extra = append(extra, "--pre-connect")
		}
		if sshConfigPath != "" {
			extra = append(extra, "--ssh-config", sshConfigPath)
		}
		opts.ExtraArgs = extra
	}

	return install.DockerEntry(opts), nil
}
