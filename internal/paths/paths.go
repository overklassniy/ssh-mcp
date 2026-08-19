// Package paths validates local and remote file paths against configured
// allowed-path lists, preventing path traversal attacks on both the local
// filesystem and the remote SFTP server.
package paths

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ValidateLocal checks that a local path is within an allowed root.
// allowedRoots is a list of resolved absolute directories. The current
// working directory is typically included.
// purpose is "read" or "write" — for writes, the parent directory must exist.
// Returns the resolved absolute path or an error describing the violation.
func ValidateLocal(localPath string, allowedRoots []string, purpose string) (string, error) {
	if localPath == "" {
		return "", fmt.Errorf("local path must be a non-empty string")
	}
	if strings.ContainsRune(localPath, 0) {
		return "", fmt.Errorf("local path must not contain null bytes")
	}

	absPath, _ := filepath.Abs(localPath)
	resolved := filepath.Clean(absPath)
	parentDir := filepath.Dir(resolved)

	// For write operations, the parent directory must exist.
	if purpose == "write" {
		if info, err := os.Stat(parentDir); err != nil || !info.IsDir() {
			return "", fmt.Errorf(
				"local path parent directory must exist and be within an allowed path. Resolved to: %s. Allowed local paths: %s",
				resolved, strings.Join(allowedRoots, ", "),
			)
		}
	}

	if !isWithinRoot(resolved, allowedRoots) {
		return "", fmt.Errorf(
			"path traversal detected. Local path resolved to: %s. Allowed local paths: %s",
			resolved, strings.Join(allowedRoots, ", "),
		)
	}
	return resolved, nil
}

// ResolveAllowedLocalRoots resolves and deduplicates the allowed local root
// directories. cwd is included as the first root. Symlinks are not expanded
// to avoid Windows 8.3 short-name path issues.
func ResolveAllowedLocalRoots(cwd string, extraPaths []string) []string {
	roots := []string{filepath.Clean(cwd)}
	for _, p := range extraPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		absP, _ := filepath.Abs(p)
		resolved := filepath.Clean(absP)
		roots = append(roots, resolved)
	}
	return roots
}

// ValidateRemote checks that a remote path is an absolute POSIX path within
// an allowed root list. If allowedRoots is empty, any absolute path is accepted.
// Returns the normalized path or an error.
func ValidateRemote(remotePath string, allowedRoots []string) (string, error) {
	if remotePath == "" {
		return "", fmt.Errorf("remote path must be a non-empty string")
	}
	if strings.ContainsRune(remotePath, 0) {
		return "", fmt.Errorf("remote path must not contain null bytes")
	}
	if !path.IsAbs(remotePath) {
		return "", fmt.Errorf("remote path must be an absolute POSIX path, got: %s", remotePath)
	}

	resolved := path.Clean(remotePath)
	if len(allowedRoots) == 0 {
		return resolved, nil
	}

	for _, root := range allowedRoots {
		root = path.Clean(root)
		if resolved == root || strings.HasPrefix(resolved, root+"/") {
			return resolved, nil
		}
	}

	return "", fmt.Errorf(
		"remote path is not within the configured allowed_remote_paths. Resolved to: %s. Allowed remote paths: %s",
		resolved, strings.Join(allowedRoots, ", "),
	)
}

// isWithinRoot checks whether candidate is equal to or inside one of the roots.
// On Windows, paths are compared case-insensitively.
func isWithinRoot(candidate string, roots []string) bool {
	candidate = filepath.Clean(candidate)
	for _, root := range roots {
		root = filepath.Clean(root)
		rel, err := filepath.Rel(root, candidate)
		if err != nil {
			continue
		}
		if rel == "." {
			return true
		}
		if !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
			return true
		}
	}
	return false
}

// Cwd returns the current working directory, or "." on error.
func Cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
