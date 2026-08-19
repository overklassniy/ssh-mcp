package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/overklassniy/ssh-mcp/internal/sshconfig"
	"github.com/pelletier/go-toml/v2"
)

// Load reads a TOML config file and returns a validated Config with
// defaults applied. The path must point to an existing file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	applyDefaults(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// FromServer creates a Config from a single ServerConfig (used by CLI
// single-server mode). Defaults are applied and the config is validated.
func FromServer(srv ServerConfig) (*Config, error) {
	cfg := &Config{
		Servers: []ServerConfig{srv},
	}
	applyDefaults(cfg)
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applyDefaults fills in zero-valued fields on each server from the
// [defaults] section, then fills remaining gaps with built-in defaults.
func applyDefaults(cfg *Config) {
	for i := range cfg.Servers {
		s := &cfg.Servers[i]
		if s.Transport == "" {
			if cfg.Defaults.Transport != "" {
				s.Transport = cfg.Defaults.Transport
			} else {
				s.Transport = DefaultTransport
			}
		}
		if s.ShellReadyTimeout.Duration == 0 {
			if cfg.Defaults.ShellReadyTimeout.Duration != 0 {
				s.ShellReadyTimeout.Duration = cfg.Defaults.ShellReadyTimeout.Duration
			} else {
				s.ShellReadyTimeout.Duration = DefaultShellReadyTimeout
			}
		}
		if s.CommandTimeout.Duration == 0 {
			if cfg.Defaults.CommandTimeout.Duration != 0 {
				s.CommandTimeout.Duration = cfg.Defaults.CommandTimeout.Duration
			} else {
				s.CommandTimeout.Duration = DefaultCommandTimeout
			}
		}
		if s.ConnectionTimeout.Duration == 0 {
			if cfg.Defaults.ConnectionTimeout.Duration != 0 {
				s.ConnectionTimeout.Duration = cfg.Defaults.ConnectionTimeout.Duration
			} else {
				s.ConnectionTimeout.Duration = DefaultConnectionTimeout
			}
		}
		if s.SFTPTimeout.Duration == 0 {
			if cfg.Defaults.SFTPTimeout.Duration != 0 {
				s.SFTPTimeout.Duration = cfg.Defaults.SFTPTimeout.Duration
			} else {
				s.SFTPTimeout.Duration = DefaultSFTPTimeout
			}
		}
		if s.MaxOutputBytes == 0 {
			if cfg.Defaults.MaxOutputBytes != 0 {
				s.MaxOutputBytes = cfg.Defaults.MaxOutputBytes
			} else {
				s.MaxOutputBytes = DefaultMaxOutputBytes
			}
		}
		if s.KeepaliveInterval.Duration == 0 {
			if cfg.Defaults.KeepaliveInterval.Duration != 0 {
				s.KeepaliveInterval.Duration = cfg.Defaults.KeepaliveInterval.Duration
			} else {
				s.KeepaliveInterval.Duration = DefaultKeepaliveInterval
			}
		}
		if s.KeepaliveCountMax == 0 {
			if cfg.Defaults.KeepaliveCountMax != 0 {
				s.KeepaliveCountMax = cfg.Defaults.KeepaliveCountMax
			} else {
				s.KeepaliveCountMax = DefaultKeepaliveCountMax
			}
		}
		if s.PTY == nil {
			b := cfg.Defaults.PTY
			s.PTY = &b
		}
		if s.Port == 0 {
			s.Port = 22
		}
	}
}

// validate checks that all server configs have required fields and valid values.
func validate(cfg *Config) error {
	if len(cfg.Servers) == 0 {
		return fmt.Errorf("no servers configured")
	}
	seen := make(map[string]bool)
	for i, s := range cfg.Servers {
		if s.Name == "" {
			return fmt.Errorf("server #%d: name is required", i)
		}
		if seen[s.Name] {
			return fmt.Errorf("duplicate server name %q", s.Name)
		}
		seen[s.Name] = true
		if s.Host == "" {
			return fmt.Errorf("server %q: host is required", s.Name)
		}
		if s.Port < 1 || s.Port > 65535 {
			return fmt.Errorf("server %q: port must be 1-65535, got %d", s.Name, s.Port)
		}
		if s.Username == "" {
			return fmt.Errorf("server %q: username is required", s.Name)
		}
		if s.Password == "" && s.PrivateKey == "" && s.Agent == "" && !s.TryKeyboard {
			return fmt.Errorf("server %q: no authentication method provided (password, private_key, agent, or try_keyboard)", s.Name)
		}
		if s.Transport != "" && s.Transport != "exec" && s.Transport != "shell" {
			return fmt.Errorf("server %q: transport must be 'exec' or 'shell', got %q", s.Name, s.Transport)
		}
		if s.MaxOutputBytes < 0 {
			return fmt.Errorf("server %q: max_output_bytes must be non-negative, got %d", s.Name, s.MaxOutputBytes)
		}
		if s.CommandTemplate != "" {
			if !strings.Contains(s.CommandTemplate, "<command>") && !strings.Contains(s.CommandTemplate, "<quotedCommand>") {
				return fmt.Errorf("server %q: command_template must contain <command> or <quotedCommand> placeholder", s.Name)
			}
		}
	}
	return nil
}

// ResolveSSHConfig looks up a host alias in ~/.ssh/config (or an explicit
// config file) and fills in missing fields on the server config.
// CLI-provided values take precedence over SSH config values.
func ResolveSSHConfig(srv *ServerConfig, sshConfigPath string) error {
	entry, err := sshconfig.Lookup(srv.Host, sshConfigPath)
	if err != nil {
		return fmt.Errorf("ssh config lookup for %q: %w", srv.Host, err)
	}
	if entry == nil {
		return nil
	}
	if entry.HostName != "" {
		srv.Host = entry.HostName
	}
	if srv.Username == "" && entry.User != "" {
		srv.Username = entry.User
	}
	if srv.Port == 0 && entry.Port != 0 {
		srv.Port = entry.Port
	}
	if srv.PrivateKey == "" && entry.IdentityFile != "" {
		srv.PrivateKey = entry.IdentityFile
	}
	return nil
}

// ExpandHome replaces a leading ~ with the home directory.
func ExpandHome(p string) string {
	if p == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// PreConnect reports whether any server has pre_connect=true.
func (c *Config) PreConnect() bool {
	for _, s := range c.Servers {
		if s.PreConnect {
			return true
		}
	}
	return false
}

// ServerNames returns the list of server names in order.
func (c *Config) ServerNames() []string {
	names := make([]string, 0, len(c.Servers))
	for _, s := range c.Servers {
		names = append(names, s.Name)
	}
	return names
}

// DefaultServerName returns the first server name, or "" if none.
func (c *Config) DefaultServerName() string {
	if len(c.Servers) == 0 {
		return ""
	}
	return c.Servers[0].Name
}

// GetServer returns the server config with the given name, or nil.
func (c *Config) GetServer(name string) *ServerConfig {
	if name == "" && len(c.Servers) > 0 {
		return &c.Servers[0]
	}
	for i := range c.Servers {
		if c.Servers[i].Name == name {
			return &c.Servers[i]
		}
	}
	return nil
}

// GetCommandTimeout returns the effective command timeout for the server.
func (s *ServerConfig) GetCommandTimeout() time.Duration {
	return s.CommandTimeout.Duration
}

// GetShellCommandTimeout returns the effective shell command timeout.
// Falls back to the command timeout if not separately configured.
func (s *ServerConfig) GetShellCommandTimeout() time.Duration {
	return s.CommandTimeout.Duration
}

// GetConnectionTimeout returns the effective connection timeout.
func (s *ServerConfig) GetConnectionTimeout() time.Duration {
	return s.ConnectionTimeout.Duration
}

// GetSFTPTimeout returns the effective SFTP timeout.
func (s *ServerConfig) GetSFTPTimeout() time.Duration {
	return s.SFTPTimeout.Duration
}

// GetShellReadyTimeout returns the effective shell ready timeout.
func (s *ServerConfig) GetShellReadyTimeout() time.Duration {
	return s.ShellReadyTimeout.Duration
}

// GetKeepaliveInterval returns the effective keepalive interval.
func (s *ServerConfig) GetKeepaliveInterval() time.Duration {
	return s.KeepaliveInterval.Duration
}

// GetMaxOutputBytes returns the effective max output bytes.
func (s *ServerConfig) GetMaxOutputBytes() int64 {
	return s.MaxOutputBytes
}

// GetPTY returns whether PTY allocation is enabled.
func (s *ServerConfig) GetPTY() bool {
	if s.PTY == nil {
		return true
	}
	return *s.PTY
}
