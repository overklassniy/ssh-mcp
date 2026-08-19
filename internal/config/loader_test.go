package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_SampleConfig(t *testing.T) {
	cfg, err := Load("../../testdata/sample.toml")
	require.NoError(t, err)
	require.Len(t, cfg.Servers, 2)

	s0 := cfg.Servers[0]
	assert.Equal(t, "dev", s0.Name)
	assert.Equal(t, "1.2.3.4", s0.Host)
	assert.Equal(t, 22, s0.Port)
	assert.Equal(t, "alice", s0.Username)
	assert.Equal(t, "secret", s0.Password)
	assert.Equal(t, "exec", s0.Transport)
	assert.Equal(t, 30*time.Second, s0.GetCommandTimeout())
	assert.Equal(t, 10*time.Second, s0.GetShellReadyTimeout())
	assert.Equal(t, int64(10485760), s0.GetMaxOutputBytes())
	assert.True(t, s0.GetPTY())
	assert.Equal(t, []string{"^ls( .*)?", "^cat .*"}, s0.Whitelist)

	s1 := cfg.Servers[1]
	assert.Equal(t, "bastion", s1.Name)
	assert.Equal(t, "shell", s1.Transport)
	assert.Equal(t, 15*time.Second, s1.GetShellReadyTimeout())
}

func TestLoad_DefaultsApplied(t *testing.T) {
	cfg, err := Load("../../testdata/sample.toml")
	require.NoError(t, err)
	// bastion does not set command_timeout; should get default 30s
	s1 := cfg.Servers[1]
	assert.Equal(t, 30*time.Second, s1.GetCommandTimeout())
	assert.Equal(t, 5*time.Minute, s1.GetSFTPTimeout())
	assert.Equal(t, 10*time.Second, s1.GetKeepaliveInterval())
	assert.Equal(t, 3, s1.KeepaliveCountMax)
}

func TestValidate_NoServers(t *testing.T) {
	err := validate(&Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no servers")
}

func TestValidate_DuplicateName(t *testing.T) {
	cfg := &Config{
		Servers: []ServerConfig{
			{Name: "dup", Host: "h", Port: 22, Username: "u", Password: "p", Transport: "exec"},
			{Name: "dup", Host: "h2", Port: 22, Username: "u2", Password: "p2", Transport: "exec"},
		},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestValidate_NoAuth(t *testing.T) {
	cfg := &Config{
		Servers: []ServerConfig{
			{Name: "s", Host: "h", Port: 22, Username: "u"},
		},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication")
}

func TestValidate_BadPort(t *testing.T) {
	cfg := &Config{
		Servers: []ServerConfig{
			{Name: "s", Host: "h", Port: 99999, Username: "u", Password: "p"},
		},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestValidate_BadTransport(t *testing.T) {
	cfg := &Config{
		Servers: []ServerConfig{
			{Name: "s", Host: "h", Port: 22, Username: "u", Password: "p", Transport: "invalid"},
		},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transport")
}

func TestValidate_BadCommandTemplate(t *testing.T) {
	cfg := &Config{
		Servers: []ServerConfig{
			{Name: "s", Host: "h", Port: 22, Username: "u", Password: "p", Transport: "exec", CommandTemplate: "no placeholder"},
		},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command_template")
}

func TestFromServer_SingleServer(t *testing.T) {
	cfg, err := FromServer(ServerConfig{
		Name:     "default",
		Host:     "1.2.3.4",
		Port:     22,
		Username: "root",
		Password: "pass",
	})
	require.NoError(t, err)
	assert.Len(t, cfg.Servers, 1)
	assert.Equal(t, "default", cfg.DefaultServerName())
	assert.Equal(t, "exec", cfg.Servers[0].Transport)
	assert.Equal(t, 22, cfg.Servers[0].Port)
}

func TestGetServer_ByName(t *testing.T) {
	cfg, err := Load("../../testdata/sample.toml")
	require.NoError(t, err)
	s := cfg.GetServer("dev")
	require.NotNil(t, s)
	assert.Equal(t, "dev", s.Name)
	assert.Nil(t, cfg.GetServer("nonexistent"))
}

func TestGetServer_EmptyNameReturnsDefault(t *testing.T) {
	cfg, err := Load("../../testdata/sample.toml")
	require.NoError(t, err)
	s := cfg.GetServer("")
	require.NotNil(t, s)
	assert.Equal(t, "dev", s.Name)
}

func TestDurationUnmarshal(t *testing.T) {
	var d Duration
	require.NoError(t, d.UnmarshalText([]byte("1h30m")))
	assert.Equal(t, 90*time.Minute, d.Duration)
}

func TestDurationUnmarshal_Invalid(t *testing.T) {
	var d Duration
	err := d.UnmarshalText([]byte("not a duration"))
	require.Error(t, err)
}

func TestExpandHome(t *testing.T) {
	assert.NotPanics(t, func() { ExpandHome("~") })
	assert.NotPanics(t, func() { ExpandHome("~/some/path") })
	assert.Equal(t, "/abs/path", ExpandHome("/abs/path"))
}
