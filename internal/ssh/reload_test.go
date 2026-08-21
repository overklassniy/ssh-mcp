package ssh

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/overklassniy/ssh-mcp/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestConfig builds a valid Config for the in-process SSH test
// server with the given server name and password.
func makeTestConfig(t *testing.T, name, password string) *config.Config {
	t.Helper()
	addr, cleanup := testSSHServer(t)
	t.Cleanup(cleanup)

	host, port, _ := net.SplitHostPort(addr)
	p := 22
	fmt.Sscanf(port, "%d", &p)

	cfg, err := config.FromServer(config.ServerConfig{
		Name:      name,
		Host:      host,
		Port:      p,
		Username:  "testuser",
		Password:  password,
		Transport: "exec",
	})
	require.NoError(t, err)
	return cfg
}

// TestReload_UnchangedPreservesConnection verifies that reloading with
// an identical server config does not close the existing connection.
func TestReload_UnchangedPreservesConnection(t *testing.T) {
	cfg := makeTestConfig(t, "srv", "testpass")

	manager := NewConnectionManager(cfg)
	defer manager.Disconnect()

	ctx := context.Background()
	client, err := manager.GetClient(ctx, "srv")
	require.NoError(t, err)
	require.True(t, client.IsConnected())

	// Reload with the same config.
	manager.Reload(cfg)

	// Connection should still be alive — GetClient returns the same
	// cached client without reconnecting.
	client2, err := manager.GetClient(ctx, "srv")
	require.NoError(t, err)
	assert.True(t, client2.IsConnected())
	assert.Same(t, client, client2, "unchanged reload should preserve the same client")
}

// TestReload_ChangedConfigInvalidatesConnection verifies that reloading
// with a modified server config closes the old connection so the next
// GetClient reconnects with the new settings.
func TestReload_ChangedConfigInvalidatesConnection(t *testing.T) {
	cfg := makeTestConfig(t, "srv", "testpass")

	manager := NewConnectionManager(cfg)
	defer manager.Disconnect()

	ctx := context.Background()
	client, err := manager.GetClient(ctx, "srv")
	require.NoError(t, err)
	require.True(t, client.IsConnected())

	// Build a new config with a different whitelist (non-connection
	// field) to trigger invalidation.
	newCfg := makeTestConfig(t, "srv", "testpass")
	newCfg.Servers[0].Whitelist = []string{`^echo .*`}

	manager.Reload(newCfg)

	// The old client should have been closed by Reload.
	assert.False(t, client.IsConnected(), "old client should be closed after config change")

	// A new connection should be established on next GetClient.
	client2, err := manager.GetClient(ctx, "srv")
	require.NoError(t, err)
	assert.True(t, client2.IsConnected())
	assert.NotSame(t, client, client2, "changed reload should create a new client")

	// The new client should reflect the updated whitelist.
	pol := manager.Policy("srv")
	require.NotNil(t, pol)
	assert.True(t, pol.HasWhitelist())
}

// TestReload_RemovedServerClosesConnection verifies that reloading
// without a previously-connected server closes its connection.
func TestReload_RemovedServerClosesConnection(t *testing.T) {
	cfg := makeTestConfig(t, "srv", "testpass")

	manager := NewConnectionManager(cfg)
	defer manager.Disconnect()

	ctx := context.Background()
	client, err := manager.GetClient(ctx, "srv")
	require.NoError(t, err)
	require.True(t, client.IsConnected())

	// Build a new config with a different server name (old one removed).
	newCfg := makeTestConfig(t, "other", "testpass")

	manager.Reload(newCfg)

	// Old client should be closed.
	assert.False(t, client.IsConnected(), "removed server connection should be closed")

	// Old server name should no longer be listed.
	names := manager.ListServers()
	assert.Contains(t, names, "other")
	assert.NotContains(t, names, "srv")
}

// TestReload_AddedServerBecomesAvailable verifies that a server added
// in the reloaded config is available for connection.
func TestReload_AddedServerBecomesAvailable(t *testing.T) {
	cfg := makeTestConfig(t, "srv", "testpass")

	manager := NewConnectionManager(cfg)
	defer manager.Disconnect()

	// Build a new config that adds a second server pointing to the
	// same test SSH server.
	srv2 := cfg.Servers[0]
	srv2.Name = "srv2"
	newCfg := &config.Config{
		Servers: []config.ServerConfig{cfg.Servers[0], srv2},
	}

	manager.Reload(newCfg)

	names := manager.ListServers()
	assert.Contains(t, names, "srv")
	assert.Contains(t, names, "srv2")

	ctx := context.Background()
	client2, err := manager.GetClient(ctx, "srv2")
	require.NoError(t, err)
	assert.True(t, client2.IsConnected())
}

// TestReload_OnClosedManagerIsIgnored verifies that Reload on a
// disconnected manager is a no-op.
func TestReload_OnClosedManagerIsIgnored(t *testing.T) {
	cfg := makeTestConfig(t, "srv", "testpass")

	manager := NewConnectionManager(cfg)
	manager.Disconnect()

	// Should not panic or cause issues.
	manager.Reload(cfg)

	// Manager should still report closed state.
	_, err := manager.GetClient(context.Background(), "srv")
	require.Error(t, err)
}

// TestReload_PolicyFailureRetainsOld verifies that if a new policy
// fails to compile, the old policy is retained for that server.
func TestReload_PolicyFailureRetainsOld(t *testing.T) {
	cfg := makeTestConfig(t, "srv", "testpass")
	cfg.Servers[0].Whitelist = []string{`^echo .*`}

	manager := NewConnectionManager(cfg)
	defer manager.Disconnect()

	oldPol := manager.Policy("srv")
	require.NotNil(t, oldPol)
	require.True(t, oldPol.HasWhitelist())

	// Reload with an invalid whitelist regex.
	newCfg := makeTestConfig(t, "srv", "testpass")
	newCfg.Servers[0].Whitelist = []string{`[invalid(`}

	manager.Reload(newCfg)

	// Old policy should be retained.
	newPol := manager.Policy("srv")
	assert.Same(t, oldPol, newPol, "old policy should be retained on compilation failure")
}

