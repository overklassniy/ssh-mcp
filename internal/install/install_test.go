package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMergeEntry_NewFile verifies that merging into an empty config creates
// the root key and the named entry.
func TestMergeEntry_NewFile(t *testing.T) {
	out, err := mergeEntry(nil, "mcpServers", "ssh-mcp", Entry{
		Command: "/usr/local/bin/ssh-mcp",
		Args:    []string{"--config", "/etc/ssh-mcp.toml"},
	})
	if err != nil {
		t.Fatalf("mergeEntry: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing or wrong type: %v", cfg["mcpServers"])
	}
	entry, ok := servers["ssh-mcp"].(map[string]any)
	if !ok {
		t.Fatalf("ssh-mcp entry missing: %v", servers)
	}
	if entry["command"] != "/usr/local/bin/ssh-mcp" {
		t.Errorf("command = %v, want /usr/local/bin/ssh-mcp", entry["command"])
	}
}

// TestMergeEntry_PreservesOtherServers verifies that merging does not
// clobber existing server entries under the same root key.
func TestMergeEntry_PreservesOtherServers(t *testing.T) {
	existing := []byte(`{
  "mcpServers": {
    "other": { "command": "npx", "args": ["-y", "other-mcp"] }
  }
}`)
	out, err := mergeEntry(existing, "mcpServers", "ssh-mcp", Entry{
		Command: "/usr/local/bin/ssh-mcp",
	})
	if err != nil {
		t.Fatalf("mergeEntry: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	servers := cfg["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Errorf("existing server 'other' was removed")
	}
	if _, ok := servers["ssh-mcp"]; !ok {
		t.Errorf("new server 'ssh-mcp' was not added")
	}
}

// TestMergeEntry_VSCodeUsesServersKey verifies the VS Code quirk: the root
// key is "servers", not "mcpServers".
func TestMergeEntry_VSCodeUsesServersKey(t *testing.T) {
	out, err := mergeEntry(nil, "servers", "ssh-mcp", Entry{Command: "ssh-mcp"})
	if err != nil {
		t.Fatalf("mergeEntry: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := cfg["servers"]; !ok {
		t.Errorf("VS Code root key 'servers' missing")
	}
	if _, ok := cfg["mcpServers"]; ok {
		t.Errorf("VS Code config must not use 'mcpServers'")
	}
}

// TestMergeEntry_OverwritesSameName verifies that re-installing under the
// same name replaces the previous entry rather than nesting it.
func TestMergeEntry_OverwritesSameName(t *testing.T) {
	existing := []byte(`{"mcpServers":{"ssh-mcp":{"command":"old"}}}`)
	out, err := mergeEntry(existing, "mcpServers", "ssh-mcp", Entry{Command: "new"})
	if err != nil {
		t.Fatalf("mergeEntry: %v", err)
	}

	var cfg map[string]any
	json.Unmarshal(out, &cfg)
	entry := cfg["mcpServers"].(map[string]any)["ssh-mcp"].(map[string]any)
	if entry["command"] != "new" {
		t.Errorf("command = %v, want new", entry["command"])
	}
}

// TestInstall_DryRun verifies that DryRun returns the content and path
// without touching the filesystem.
func TestInstall_DryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	res, err := Install(InstallOptions{
		Client:              ClientCursor,
		Name:                "ssh-mcp",
		Entry:               Entry{Command: "ssh-mcp", Args: []string{"--config", "x.toml"}},
		ConfigPathOverride:  path,
		DryRun:              true,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Written {
		t.Errorf("DryRun must not write")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("DryRun must not create the file, got %v", err)
	}
	if len(res.Content) == 0 {
		t.Errorf("DryRun must return content")
	}
}

// TestInstall_WritesAndBacksUp verifies that a real install creates the
// file, merges, and backs up the previous content.
func TestInstall_WritesAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	original := []byte(`{"mcpServers":{"other":{"command":"npx"}}}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := Install(InstallOptions{
		Client:             ClientCursor,
		Name:               "ssh-mcp",
		Entry:              Entry{Command: "ssh-mcp"},
		ConfigPathOverride: path,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !res.Written {
		t.Errorf("expected Written=true")
	}
	if res.BackupPath == "" {
		t.Errorf("expected a backup path for an existing file")
	}
	backup, err := os.ReadFile(res.BackupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != string(original) {
		t.Errorf("backup content mismatch")
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written: %v", err)
	}
	var cfg map[string]any
	json.Unmarshal(written, &cfg)
	servers := cfg["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Errorf("existing 'other' server lost")
	}
	if _, ok := servers["ssh-mcp"]; !ok {
		t.Errorf("new 'ssh-mcp' server missing")
	}
}

// TestInstall_CreatesParentDirs verifies that missing parent directories
// are created.
func TestInstall_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "mcp.json")
	_, err := Install(InstallOptions{
		Client:             ClientCursor,
		Name:               "ssh-mcp",
		Entry:              Entry{Command: "ssh-mcp"},
		ConfigPathOverride: path,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

// TestRootKey_VSCode verifies the VS Code root key is "servers".
func TestRootKey_VSCode(t *testing.T) {
	k, err := RootKey(ClientVSCode)
	if err != nil {
		t.Fatalf("RootKey: %v", err)
	}
	if k != "servers" {
		t.Errorf("RootKey(vscode) = %q, want servers", k)
	}
}

// TestRootKey_OthersUseMcpServers verifies all non-VS-Code clients use
// "mcpServers".
func TestRootKey_OthersUseMcpServers(t *testing.T) {
	for _, c := range []Client{ClientClaudeDesktop, ClientClaudeCode, ClientCursor, ClientDevin, ClientCline} {
		k, err := RootKey(c)
		if err != nil {
			t.Fatalf("RootKey(%s): %v", c, err)
		}
		if k != "mcpServers" {
			t.Errorf("RootKey(%s) = %q, want mcpServers", c, k)
		}
	}
}

// TestConfigPath_ResolvesPerOS verifies that ConfigPath returns a non-empty
// path ending in the expected filename for each client on the current OS.
func TestConfigPath_ResolvesPerOS(t *testing.T) {
	want := map[Client]string{
		ClientClaudeDesktop: "claude_desktop_config.json",
		ClientClaudeCode:    "settings.json",
		ClientCursor:        "mcp.json",
		ClientDevin:      "mcp_config.json",
		ClientVSCode:        "mcp.json",
		ClientCline:         "cline_mcp_settings.json",
	}
	for c, name := range want {
		p, err := ConfigPath(c)
		if err != nil {
			t.Fatalf("ConfigPath(%s): %v", c, err)
		}
		if filepath.Base(p) != name {
			t.Errorf("ConfigPath(%s) base = %q, want %q (full=%s, goos=%s)", c, filepath.Base(p), name, p, runtime.GOOS)
		}
	}
}

// TestSpec_UnknownClient verifies an unknown client returns an error.
func TestSpec_UnknownClient(t *testing.T) {
	if _, err := RootKey(Client("nope")); err == nil {
		t.Errorf("expected error for unknown client")
	}
}

// TestSupportedClients_Alphabetical verifies the list is sorted and
// contains all six clients.
func TestSupportedClients_Alphabetical(t *testing.T) {
	cs := SupportedClients()
	if len(cs) != 6 {
		t.Errorf("expected 6 clients, got %d", len(cs))
	}
	for i := 1; i < len(cs); i++ {
		if cs[i-1] > cs[i] {
			t.Errorf("clients not sorted: %v", cs)
		}
	}
}

// TestDockerEntry_ConfigMode verifies the docker entry shape when a TOML
// config file is provided: docker run, env passthrough, config volume
// mount, and --config /config.toml after the image.
func TestDockerEntry_ConfigMode(t *testing.T) {
	entry := DockerEntry(DockerEntryOptions{
		Image:      "ghcr.io/overklassniy/ssh-mcp:dev",
		ConfigPath: "/home/alice/ssh-mcp.toml",
		Home:       "/home/alice",
	})

	if entry.Command != "docker" {
		t.Fatalf("command = %q, want docker", entry.Command)
	}
	wantPrefix := []string{
		"run", "-i", "--rm",
		"-e", "SSH_MCP_PASSWORD",
		"-e", "SSH_MCP_PASSPHRASE",
		"-e", "SSH_MCP_2FA_CODE",
		"-e", "HOME=/home/alice",
		"-v", "/home/alice:/home/alice",
		"-v", "/home/alice/ssh-mcp.toml:/config.toml:ro",
		"ghcr.io/overklassniy/ssh-mcp:dev",
		"--config", "/config.toml",
	}
	if len(entry.Args) < len(wantPrefix) {
		t.Fatalf("args too short: %v", entry.Args)
	}
	for i, w := range wantPrefix {
		if entry.Args[i] != w {
			t.Errorf("args[%d] = %q, want %q (full=%v)", i, entry.Args[i], w, entry.Args)
		}
	}
	if len(entry.Args) != len(wantPrefix) {
		t.Errorf("unexpected trailing args: %v", entry.Args[len(wantPrefix):])
	}
}

// TestDockerEntry_AgentSocket verifies the SSH agent socket is mounted and
// SSH_AUTH_SOCK is forwarded.
func TestDockerEntry_AgentSocket(t *testing.T) {
	entry := DockerEntry(DockerEntryOptions{
		AgentSocket: "/tmp/ssh-agent.sock",
		ExtraArgs:   []string{"--host", "1.2.3.4", "--user", "alice"},
	})

	joined := joinArgs(entry.Args)
	if !contains(entry.Args, "-e", "SSH_AUTH_SOCK") {
		t.Errorf("expected -e SSH_AUTH_SOCK, got %v", entry.Args)
	}
	if !contains(entry.Args, "-v", "/tmp/ssh-agent.sock:/tmp/ssh-agent.sock") {
		t.Errorf("expected agent socket volume mount, got %v", entry.Args)
	}
	if !contains(entry.Args, "--host", "1.2.3.4") {
		t.Errorf("expected extra args after image, got %s", joined)
	}
}

// TestDockerEntry_DefaultImage verifies that an empty image falls back to
// DefaultDockerImage.
func TestDockerEntry_DefaultImage(t *testing.T) {
	entry := DockerEntry(DockerEntryOptions{})
	found := false
	for _, a := range entry.Args {
		if a == DefaultDockerImage {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected default image %q in args, got %v", DefaultDockerImage, entry.Args)
	}
}

// TestDockerEntry_NoHomeNoAgent verifies the minimal entry omits home and
// agent mounts when not provided.
func TestDockerEntry_NoHomeNoAgent(t *testing.T) {
	entry := DockerEntry(DockerEntryOptions{
		ConfigPath: "/etc/ssh-mcp.toml",
	})
	for i, a := range entry.Args {
		if a == "-e" && i+1 < len(entry.Args) && entry.Args[i+1] == "HOME=" {
			t.Errorf("HOME should not be set when Home is empty")
		}
		if a == "SSH_AUTH_SOCK" {
			t.Errorf("SSH_AUTH_SOCK should not be set when AgentSocket is empty")
		}
	}
}

// joinArgs concatenates args with spaces for readable failure messages.
func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

// contains checks whether the pair (flag, value) appears consecutively in
// args.
func contains(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}
