package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLocal_WithinCwd(t *testing.T) {
	dir := t.TempDir()
	roots := []string{dir}
	target := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0644))
	resolved, err := ValidateLocal(target, roots, "read")
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(target), resolved)
}

func TestValidateLocal_TraversalRejected(t *testing.T) {
	dir := t.TempDir()
	roots := []string{dir}
	outside := filepath.Join(filepath.Dir(dir), "other")
	_, err := ValidateLocal(outside, roots, "read")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "traversal")
}

func TestValidateLocal_EmptyPath(t *testing.T) {
	_, err := ValidateLocal("", []string{"/"}, "read")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty")
}

func TestValidateLocal_NullBytes(t *testing.T) {
	_, err := ValidateLocal("a\x00b", []string{"/"}, "read")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "null")
}

func TestValidateLocal_WriteMissingParent(t *testing.T) {
	dir := t.TempDir()
	roots := []string{dir}
	missing := filepath.Join(dir, "nonexistent_dir", "file.txt")
	_, err := ValidateLocal(missing, roots, "write")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parent directory")
}

func TestValidateRemote_AbsolutePath(t *testing.T) {
	resolved, err := ValidateRemote("/var/log/app.log", nil)
	require.NoError(t, err)
	assert.Equal(t, "/var/log/app.log", resolved)
}

func TestValidateRemote_WithinRoot(t *testing.T) {
	resolved, err := ValidateRemote("/var/log/app.log", []string{"/var/log"})
	require.NoError(t, err)
	assert.Equal(t, "/var/log/app.log", resolved)
}

func TestValidateRemote_OutsideRoot(t *testing.T) {
	_, err := ValidateRemote("/etc/passwd", []string{"/var/log"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allowed_remote_paths")
}

func TestValidateRemote_RelativeRejected(t *testing.T) {
	_, err := ValidateRemote("relative/path", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute")
}

func TestValidateRemote_Normalized(t *testing.T) {
	resolved, err := ValidateRemote("/var/log/../log/./app.log", nil)
	require.NoError(t, err)
	assert.Equal(t, "/var/log/app.log", resolved)
}

func TestResolveAllowedLocalRoots(t *testing.T) {
	dir := t.TempDir()
	roots := ResolveAllowedLocalRoots(dir, []string{dir, ""})
	assert.Contains(t, roots, dir)
}

func TestCwd(t *testing.T) {
	assert.NotPanics(t, func() { Cwd() })
}
