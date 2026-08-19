// Package config defines the TOML configuration schema for ssh-mcp
// and provides loading, validation, and merging of CLI flags.
package config

import (
	"time"
)

// Config is the top-level configuration loaded from a TOML file or CLI flags.
type Config struct {
	Defaults Defaults       `toml:"defaults"`
	Servers  []ServerConfig `toml:"server"`
}

// Defaults holds global default values inherited by every server
// unless the server overrides them.
type Defaults struct {
	CommandTimeout    Duration `toml:"command_timeout"`
	ConnectionTimeout Duration `toml:"connection_timeout"`
	SFTPTimeout       Duration `toml:"sftp_timeout"`
	ShellReadyTimeout Duration `toml:"shell_ready_timeout"`
	MaxOutputBytes    int64    `toml:"max_output_bytes"`
	KeepaliveInterval Duration `toml:"keepalive_interval"`
	KeepaliveCountMax int      `toml:"keepalive_count_max"`
	PTY               bool     `toml:"pty"`
	Transport         string   `toml:"transport"`
}

// ServerConfig describes a single SSH target. The Name field is the
// connectionName used in MCP tool calls.
type ServerConfig struct {
	Name              string   `toml:"name"`
	Host              string   `toml:"host"`
	Port              int      `toml:"port"`
	Username          string   `toml:"username"`
	Password          string   `toml:"password,omitempty"`
	PrivateKey        string   `toml:"private_key,omitempty"`
	Passphrase        string   `toml:"passphrase,omitempty"`
	Agent             string   `toml:"agent,omitempty"`
	Proxy             string   `toml:"proxy,omitempty"`
	Transport         string   `toml:"transport,omitempty"`
	ShellReadyTimeout Duration `toml:"shell_ready_timeout,omitempty"`
	CommandTimeout    Duration `toml:"command_timeout,omitempty"`
	ConnectionTimeout Duration `toml:"connection_timeout,omitempty"`
	SFTPTimeout       Duration `toml:"sftp_timeout,omitempty"`
	MaxOutputBytes    int64    `toml:"max_output_bytes,omitempty"`
	KeepaliveInterval Duration `toml:"keepalive_interval,omitempty"`
	KeepaliveCountMax int      `toml:"keepalive_count_max,omitempty"`
	PTY               *bool    `toml:"pty,omitempty"`
	TryKeyboard       bool     `toml:"try_keyboard,omitempty"`
	PreConnect        bool     `toml:"pre_connect,omitempty"`
	Whitelist         []string `toml:"whitelist,omitempty"`
	Blacklist         []string `toml:"blacklist,omitempty"`
	AllowedLocalPaths  []string `toml:"allowed_local_paths,omitempty"`
	AllowedRemotePaths []string `toml:"allowed_remote_paths,omitempty"`
	CommandTemplate   string   `toml:"command_template,omitempty"`
}

// Duration wraps time.Duration so TOML can decode duration strings
// like "30s", "5m", "1h30m" via time.ParseDuration.
type Duration struct {
	time.Duration
}

// UnmarshalText implements encoding.TextUnmarshaler for TOML decoding.
func (d *Duration) UnmarshalText(text []byte) error {
	v, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

// MarshalText implements encoding.TextMarshaler for TOML encoding.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// Default values applied when neither the server nor [defaults] provides one.
const (
	DefaultCommandTimeout    = 30 * time.Second
	DefaultConnectionTimeout = 30 * time.Second
	DefaultSFTPTimeout       = 5 * time.Minute
	DefaultShellReadyTimeout = 10 * time.Second
	DefaultMaxOutputBytes    = 10 * 1024 * 1024 // 10 MiB
	DefaultKeepaliveInterval = 10 * time.Second
	DefaultKeepaliveCountMax = 3
	DefaultTransport         = "exec"
)
