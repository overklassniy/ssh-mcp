package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeConfig writes a minimal valid TOML config to path and returns
// the mtime after the write.
func writeConfig(t *testing.T, path, serverName string) time.Time {
	t.Helper()
	content := `[[server]]
name = "` + serverName + `"
host = "127.0.0.1"
port = 22
username = "testuser"
password = "secret"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	fi, err := os.Stat(path)
	require.NoError(t, err)
	return fi.ModTime()
}

func TestWatcher_DetectsChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	writeConfig(t, path, "alpha")

	w := NewWatcher(path, 50*time.Millisecond)
	ch := w.Start()
	defer w.Stop()

	// Wait for the watcher to record the initial mtime.
	time.Sleep(100 * time.Millisecond)

	// Modify the file. On some filesystems the mtime resolution is
	// coarse (1s), so we sleep before rewriting to guarantee the
	// mtime advances.
	time.Sleep(1100 * time.Millisecond)
	writeConfig(t, path, "beta")

	select {
	case cfg := <-ch:
		require.NotNil(t, cfg)
		require.Len(t, cfg.Servers, 1)
		assert.Equal(t, "beta", cfg.Servers[0].Name)
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not emit reloaded config within 3s")
	}
}

func TestWatcher_IgnoresUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	writeConfig(t, path, "alpha")

	w := NewWatcher(path, 50*time.Millisecond)
	ch := w.Start()
	defer w.Stop()

	// Wait past a few poll cycles without modifying the file.
	select {
	case cfg := <-ch:
		t.Fatalf("watcher emitted unexpected config: %v", cfg)
	case <-time.After(250 * time.Millisecond):
		// No change detected — expected.
	}
}

func TestWatcher_KeepsOldConfigOnParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	writeConfig(t, path, "alpha")

	w := NewWatcher(path, 50*time.Millisecond)
	ch := w.Start()
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Write an invalid TOML file. The watcher should log the error
	// and NOT emit anything on the channel.
	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, os.WriteFile(path, []byte("not valid toml {{{"), 0644))

	select {
	case cfg := <-ch:
		t.Fatalf("watcher emitted config despite parse error: %v", cfg)
	case <-time.After(300 * time.Millisecond):
		// No emit — expected.
	}
}

func TestWatcher_StopClosesChannel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")
	writeConfig(t, path, "alpha")

	w := NewWatcher(path, 50*time.Millisecond)
	ch := w.Start()

	w.Stop()

	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after Stop")
}

func TestWatcher_MissingFileAtStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.toml")

	w := NewWatcher(path, 50*time.Millisecond)
	ch := w.Start()
	defer w.Stop()

	// Create the file after the watcher starts.
	time.Sleep(100 * time.Millisecond)
	writeConfig(t, path, "gamma")

	select {
	case cfg := <-ch:
		require.NotNil(t, cfg)
		require.Len(t, cfg.Servers, 1)
		assert.Equal(t, "gamma", cfg.Servers[0].Name)
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not detect newly created file within 3s")
	}
}
