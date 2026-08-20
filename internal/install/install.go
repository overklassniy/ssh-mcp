// Package install writes ssh-mcp server entries into the MCP configuration
// files of supported AI clients (Claude Desktop, Claude Code, Cursor,
// Devin, VS Code, and Cline). It resolves the correct config file path
// and root key per client and operating system, then merges the new entry
// non-destructively alongside any existing servers.
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// Client identifies a supported AI client application.
type Client string

const (
	ClientClaudeDesktop Client = "claude-desktop"
	ClientClaudeCode    Client = "claude-code"
	ClientCursor        Client = "cursor"
	ClientDevin      Client = "Devin"
	ClientVSCode        Client = "vscode"
	ClientCline         Client = "cline"
)

// SupportedClients returns the list of clients the installer can target,
// sorted alphabetically for stable display.
func SupportedClients() []Client {
	clients := []Client{
		ClientClaudeDesktop,
		ClientClaudeCode,
		ClientCursor,
		ClientDevin,
		ClientVSCode,
		ClientCline,
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i] < clients[j] })
	return clients
}

// clientSpec describes where a client stores its MCP config and which root
// JSON key it uses. pathFor returns the absolute config file path for the
// given OS, home directory, and per-OS app-data/config directory.
type clientSpec struct {
	rootKey string
	pathFor func(goos, home, appConfigDir string) string
}

// spec returns the clientSpec for the given Client, or an error if the
// client is unknown.
func spec(c Client) (clientSpec, error) {
	switch c {
	case ClientClaudeDesktop:
		return clientSpec{
			rootKey: "mcpServers",
			pathFor: func(goos, home, appConfigDir string) string {
				switch goos {
				case "windows":
					return filepath.Join(appConfigDir, "Claude", "claude_desktop_config.json")
				case "darwin":
					return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
				default:
					return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
				}
			},
		}, nil
	case ClientClaudeCode:
		return clientSpec{
			rootKey: "mcpServers",
			pathFor: func(_, home, _ string) string {
				return filepath.Join(home, ".claude", "settings.json")
			},
		}, nil
	case ClientCursor:
		return clientSpec{
			rootKey: "mcpServers",
			pathFor: func(_, home, _ string) string {
				return filepath.Join(home, ".cursor", "mcp.json")
			},
		}, nil
	case ClientDevin:
		return clientSpec{
			rootKey: "mcpServers",
			pathFor: func(_, home, _ string) string {
				return filepath.Join(home, ".codeium", "Devin", "mcp_config.json")
			},
		}, nil
	case ClientVSCode:
		// VS Code is the only major client that uses "servers" as the root
		// key instead of "mcpServers". Copying a config from another client
		// into .vscode/mcp.json will silently fail without this.
		return clientSpec{
			rootKey: "servers",
			pathFor: func(_, home, _ string) string {
				return filepath.Join(home, ".vscode", "mcp.json")
			},
		}, nil
	case ClientCline:
		return clientSpec{
			rootKey: "mcpServers",
			pathFor: func(goos, home, appConfigDir string) string {
				rel := filepath.Join("User", "globalStorage", "saoudrizwan.claude-dev", "settings", "cline_mcp_settings.json")
				switch goos {
				case "windows":
					return filepath.Join(appConfigDir, "Code", rel)
				case "darwin":
					return filepath.Join(home, "Library", "Application Support", "Code", rel)
				default:
					return filepath.Join(home, ".config", "Code", rel)
				}
			},
		}, nil
	default:
		return clientSpec{}, fmt.Errorf("unknown client %q; supported: %s", c, joinClients(SupportedClients()))
	}
}

// joinClients renders a comma-separated list of client names for error
// messages.
func joinClients(cs []Client) string {
	out := ""
	for i, c := range cs {
		if i > 0 {
			out += ", "
		}
		out += string(c)
	}
	return out
}

// Entry is the MCP server entry written under the client's root key.
type Entry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// ConfigPath returns the absolute config file path the installer would
// target for the given client on the current operating system. It does not
// require the file to exist.
func ConfigPath(c Client) (string, error) {
	s, err := spec(c)
	if err != nil {
		return "", err
	}
	return resolvePath(s.pathFor), nil
}

// RootKey returns the JSON root key ("mcpServers" or "servers") the given
// client uses for its MCP server entries.
func RootKey(c Client) (string, error) {
	s, err := spec(c)
	if err != nil {
		return "", err
	}
	return s.rootKey, nil
}

// resolvePath calls the pathFor function with the current OS, home dir, and
// per-OS app config directory.
func resolvePath(pathFor func(goos, home, appConfigDir string) string) string {
	home, _ := os.UserHomeDir()
	appConfigDir := perOSAppConfigDir()
	return pathFor(runtime.GOOS, home, appConfigDir)
}

// perOSAppConfigDir returns the directory that holds per-user application
// configuration on the current OS: %APPDATA% on Windows,
// ~/Library/Application Support on macOS (handled by pathFor via home), and
// ~/.config on Linux (handled by pathFor via home). For Windows it resolves
// %APPDATA% directly.
func perOSAppConfigDir() string {
	if runtime.GOOS == "windows" {
		if v := os.Getenv("APPDATA"); v != "" {
			return v
		}
	}
	home, _ := os.UserHomeDir()
	return home
}

// InstallOptions controls how an entry is written.
type InstallOptions struct {
	// Client is the target AI client.
	Client Client
	// Name is the alias under which the server is registered. Defaults to
	// "ssh-mcp" when empty.
	Name string
	// Entry is the server entry to write.
	Entry Entry
	// ConfigPathOverride lets callers (and tests) redirect the target file.
	// When empty, the path is resolved from the client spec.
	ConfigPathOverride string
	// DryRun skips writing to disk and returns the JSON that would be
	// written plus the resolved path.
	DryRun bool
}

// Result describes what the installer did.
type Result struct {
	// Path is the config file that was written (or would be, on dry run).
	Path string
	// Written reports whether the file was actually modified.
	Written bool
	// BackupPath is the path of the backup file created before writing, or
	// empty when no backup was needed (new file) or on dry run.
	BackupPath string
	// Content is the JSON bytes written (or that would be written on dry
	// run).
	Content []byte
}

// Install writes the entry into the client's config file, creating the file
// and parent directories as needed and merging non-destructively under the
// client's root key. A backup of the existing file is created before
// writing.
func Install(opts InstallOptions) (*Result, error) {
	if opts.Name == "" {
		opts.Name = "ssh-mcp"
	}
	s, err := spec(opts.Client)
	if err != nil {
		return nil, err
	}

	path := opts.ConfigPathOverride
	if path == "" {
		path = resolvePath(s.pathFor)
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
		existing = nil
	}

	merged, err := mergeEntry(existing, s.rootKey, opts.Name, opts.Entry)
	if err != nil {
		return nil, fmt.Errorf("merge entry into %s: %w", path, err)
	}

	res := &Result{Path: path, Content: merged}

	if opts.DryRun {
		return res, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err)
	}

	if existing != nil {
		backup := path + ".bak"
		if err := os.WriteFile(backup, existing, 0o644); err != nil {
			return nil, fmt.Errorf("write backup %s: %w", backup, err)
		}
		res.BackupPath = backup
	}

	if err := os.WriteFile(path, merged, 0o644); err != nil {
		return nil, fmt.Errorf("write config %s: %w", path, err)
	}
	res.Written = true
	return res, nil
}

// DefaultDockerImage is the container image used by DockerEntry when no
// image is explicitly provided. Points at the GHCR primary registry.
const DefaultDockerImage = "ghcr.io/overklassniy/ssh-mcp:latest"

// DockerEntryOptions controls how a docker-based server entry is built.
type DockerEntryOptions struct {
	// Image is the container image to run. Defaults to DefaultDockerImage.
	Image string
	// ConfigPath is the host-side path to the TOML config file. It is
	// mounted read-only at /config.toml inside the container. When empty,
	// single-server CLI flags are expected in ExtraArgs instead.
	ConfigPath string
	// Home is the host home directory. It is bind-mounted at the same
	// absolute path inside the container so that ~/ paths in the config
	// resolve identically. May be empty to skip the home mount.
	Home string
	// AgentSocket is the host path of the SSH agent socket (the value of
	// $SSH_AUTH_SOCK). When non-empty, it is bind-mounted at the same path
	// and SSH_AUTH_SOCK is set in the container env. May be empty.
	AgentSocket string
	// ExtraArgs are additional ssh-mcp CLI flags appended after the image
	// name, used in single-server mode (e.g. --host, --user, ...).
	ExtraArgs []string
}

// DockerEntry builds an Entry that launches ssh-mcp inside a container via
// `docker run -i --rm`. The container receives SSH_MCP_PASSWORD,
// SSH_MCP_PASSPHRASE, and SSH_MCP_2FA_CODE from the host environment, and
// optionally the SSH agent socket and the host home directory for
// transparent path resolution.
//
// The resulting Entry.Command is "docker" and Entry.Args is the full
// `run` argument list. Sensitive values stay in the host environment and
// are forwarded via -e, so they never appear in the args.
func DockerEntry(opts DockerEntryOptions) Entry {
	image := opts.Image
	if image == "" {
		image = DefaultDockerImage
	}

	args := []string{
		"run", "-i", "--rm",
		"-e", "SSH_MCP_PASSWORD",
		"-e", "SSH_MCP_PASSPHRASE",
		"-e", "SSH_MCP_2FA_CODE",
	}

	if opts.AgentSocket != "" {
		args = append(args,
			"-e", "SSH_AUTH_SOCK",
			"-v", opts.AgentSocket+":"+opts.AgentSocket,
		)
	}

	if opts.Home != "" {
		args = append(args,
			"-e", "HOME="+opts.Home,
			"-v", opts.Home+":"+opts.Home,
		)
	}

	// Server-side args appended after the image name.
	serverArgs := opts.ExtraArgs
	if opts.ConfigPath != "" {
		args = append(args, "-v", opts.ConfigPath+":/config.toml:ro")
		serverArgs = append([]string{"--config", "/config.toml"}, serverArgs...)
	}

	args = append(args, image)
	args = append(args, serverArgs...)

	return Entry{Command: "docker", Args: args}
}

// DefaultGoRunTarget is the module path appended after `go run` in
// GoRunEntry. It points at the public module's main package so that
// `go run ...@latest` fetches and builds the latest tagged release without
// a local checkout.
const DefaultGoRunTarget = "github.com/overklassniy/ssh-mcp/cmd/ssh-mcp@latest"

// GoRunEntryOptions controls how a `go run`-based server entry is built.
type GoRunEntryOptions struct {
	// Target is the module path (with optional @version) appended after
	// `go run`. Defaults to DefaultGoRunTarget.
	Target string
	// ExtraArgs are ssh-mcp CLI flags appended after the target, used in
	// single-server mode (e.g. --host, --user, ...).
	ExtraArgs []string
	// Password and Passphrase are placed in the env map (SSH_MCP_PASSWORD,
	// SSH_MCP_PASSPHRASE) so they never appear in args. Either may be
	// empty; an empty value is omitted from the env map.
	Password string
	// Passphrase is the private key passphrase, forwarded via env.
	Passphrase string
}

// GoRunEntry builds an Entry that launches ssh-mcp via `go run
// <target>` using the public Go module. This is the Go-native equivalent
// of `npx -y <package>`: it fetches and compiles the latest tagged release
// on first run, then starts the stdio MCP server.
//
// Sensitive values (password, passphrase) are placed in the env map rather
// than args so they are not visible in process argument listings. The
// caller is responsible for ensuring the Go toolchain is installed.
func GoRunEntry(opts GoRunEntryOptions) Entry {
	target := opts.Target
	if target == "" {
		target = DefaultGoRunTarget
	}

	args := append([]string{"run", target}, opts.ExtraArgs...)

	env := map[string]string{}
	if opts.Password != "" {
		env["SSH_MCP_PASSWORD"] = opts.Password
	}
	if opts.Passphrase != "" {
		env["SSH_MCP_PASSPHRASE"] = opts.Passphrase
	}

	return Entry{Command: "go", Args: args, Env: env}
}

// mergeEntry parses the existing config bytes (which may be empty or
// missing), ensures the root key object exists, sets the named server entry
// under it, and returns the pretty-printed JSON. Other top-level keys and
// other server entries are preserved.
func mergeEntry(existing []byte, rootKey, name string, entry Entry) ([]byte, error) {
	var cfg map[string]any
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &cfg); err != nil {
			return nil, fmt.Errorf("parse existing JSON: %w", err)
		}
	}
	if cfg == nil {
		cfg = map[string]any{}
	}

	servers, ok := cfg[rootKey].(map[string]any)
	if !ok || servers == nil {
		servers = map[string]any{}
		cfg[rootKey] = servers
	}

	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal entry: %w", err)
	}
	var entryMap map[string]any
	if err := json.Unmarshal(entryBytes, &entryMap); err != nil {
		return nil, fmt.Errorf("unmarshal entry map: %w", err)
	}
	servers[name] = entryMap

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return append(out, '\n'), nil
}
