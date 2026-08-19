package sshconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func TestLookup_BasicHostAlias(t *testing.T) {
	cfg := writeConfig(t, `
Host myserver
    HostName 192.168.1.1
    Port 22
    User root
    IdentityFile ~/.ssh/id_rsa
`)
	entry, err := Lookup("myserver", cfg)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "192.168.1.1", entry.HostName)
	assert.Equal(t, "root", entry.User)
	assert.Equal(t, 22, entry.Port)
	assert.Contains(t, entry.IdentityFile, "id_rsa")
}

func TestLookup_MultiAliasHostLine(t *testing.T) {
	cfg := writeConfig(t, `
Host a b c
    HostName 10.0.0.1
    User shared
`)
	for _, alias := range []string{"a", "b", "c"} {
		entry, err := Lookup(alias, cfg)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.Equal(t, "10.0.0.1", entry.HostName)
		assert.Equal(t, "shared", entry.User)
	}
}

func TestLookup_WildcardPattern(t *testing.T) {
	cfg := writeConfig(t, `
Host *.example.com
    HostName %h
    User wildcard
`)
	entry, err := Lookup("foo.example.com", cfg)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "wildcard", entry.User)
}

func TestLookup_StarDefaultFallback(t *testing.T) {
	cfg := writeConfig(t, `
Host *
    User defaultuser
`)
	entry, err := Lookup("anything", cfg)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "defaultuser", entry.User)
}

func TestLookup_FirstMatchWins(t *testing.T) {
	cfg := writeConfig(t, `
Host myhost
    User first

Host myhost
    User second
`)
	entry, err := Lookup("myhost", cfg)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "first", entry.User)
}

func TestLookup_NegatedPattern(t *testing.T) {
	cfg := writeConfig(t, `
Host * !blocked
    User allowed
`)
	entry, err := Lookup("blocked", cfg)
	require.NoError(t, err)
	assert.Nil(t, entry)
	entry, err = Lookup("other", cfg)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "allowed", entry.User)
}

func TestLookup_IncludeDirective(t *testing.T) {
	dir := t.TempDir()
	mainCfg := filepath.Join(dir, "config")
	incCfg := filepath.Join(dir, "included")
	require.NoError(t, os.WriteFile(incCfg, []byte("Host included\n    HostName 5.6.7.8\n    User inc\n"), 0644))
	require.NoError(t, os.WriteFile(mainCfg, []byte("Include included\nHost main\n    HostName 1.2.3.4\n"), 0644))

	entry, err := Lookup("included", mainCfg)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "5.6.7.8", entry.HostName)
	assert.Equal(t, "inc", entry.User)

	entry, err = Lookup("main", mainCfg)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "1.2.3.4", entry.HostName)
}

func TestLookup_NotFound(t *testing.T) {
	cfg := writeConfig(t, "Host real\n    HostName 1.1.1.1\n")
	entry, err := Lookup("nonexistent", cfg)
	require.NoError(t, err)
	assert.Nil(t, entry)
}

func TestLookup_CommentsStripped(t *testing.T) {
	cfg := writeConfig(t, `
# This is a comment
Host myhost # inline comment
    HostName 2.2.2.2 # another comment
    User commented
`)
	entry, err := Lookup("myhost", cfg)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "2.2.2.2", entry.HostName)
	assert.Equal(t, "commented", entry.User)
}

func TestLookup_ExplicitMissingFile(t *testing.T) {
	_, err := Lookup("host", "/nonexistent/path/config")
	require.Error(t, err)
}
