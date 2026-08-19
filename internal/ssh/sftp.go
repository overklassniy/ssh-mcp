package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/overklassniy/ssh-mcp/internal/paths"
	"github.com/pkg/sftp"
)

// SFTPClient wraps an sftp.Client with concurrent transfer support
// and path validation.
type SFTPClient struct {
	client   *sftp.Client
	localRoots []string
	remoteRoots []string
}

// NewSFTPClient creates a new SFTP wrapper from an SSH client.
func NewSFTPClient(sshClient SSHClient) (*SFTPClient, error) {
	if !sshClient.IsConnected() {
		return nil, NewToolError(CodeSSHConnectionFailed, "not connected", true)
	}
	sc, err := sftp.NewClient(sshClient.Client())
	if err != nil {
		return nil, NewToolError(CodeSFTPError, fmt.Sprintf("create SFTP client: %v", err), true)
	}

	srv := sshClient.Config()
	cwd := paths.Cwd()
	localRoots := paths.ResolveAllowedLocalRoots(cwd, srv.AllowedLocalPaths)

	return &SFTPClient{
		client:      sc,
		localRoots:  localRoots,
		remoteRoots: srv.AllowedRemotePaths,
	}, nil
}

// Close closes the SFTP client.
func (s *SFTPClient) Close() error {
	return s.client.Close()
}

// Upload transfers a local file to the remote server using concurrent SFTP.
func (s *SFTPClient) Upload(ctx context.Context, localPath, remotePath string) error {
	// Validate local path for reading
	resolvedLocal, err := paths.ValidateLocal(localPath, s.localRoots, "read")
	if err != nil {
		return NewToolError(CodeLocalPathNotAllowed, err.Error(), false)
	}

	// Validate remote path
	resolvedRemote, err := paths.ValidateRemote(remotePath, s.remoteRoots)
	if err != nil {
		return NewToolError(CodeRemotePathNotAllowed, err.Error(), false)
	}

	// Open local file
	localFile, err := os.Open(resolvedLocal)
	if err != nil {
		return NewToolError(CodeLocalFileReadFailed, err.Error(), false)
	}
	defer localFile.Close()

	// Ensure remote directory exists
	remoteDir := path.Dir(resolvedRemote)
	if remoteDir != "." && remoteDir != "/" {
		if err := s.mkdirAll(remoteDir); err != nil {
			return NewToolError(CodeSFTPError, fmt.Sprintf("create remote directory: %v", err), true)
		}
	}

	// Create remote file
	remoteFile, err := s.client.Create(resolvedRemote)
	if err != nil {
		return NewToolError(CodeSFTPError, fmt.Sprintf("create remote file: %v", err), true)
	}
	defer remoteFile.Close()

	// Copy with concurrent writes
	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return NewToolError(CodeSFTPError, fmt.Sprintf("upload data: %v", err), true)
	}

	return nil
}

// Download transfers a remote file to the local machine using concurrent SFTP.
// It downloads to a temporary file first, then atomically renames it.
func (s *SFTPClient) Download(ctx context.Context, remotePath, localPath string) error {
	// Validate remote path
	resolvedRemote, err := paths.ValidateRemote(remotePath, s.remoteRoots)
	if err != nil {
		return NewToolError(CodeRemotePathNotAllowed, err.Error(), false)
	}

	// Validate local path for writing
	resolvedLocal, err := paths.ValidateLocal(localPath, s.localRoots, "write")
	if err != nil {
		return NewToolError(CodeLocalPathNotAllowed, err.Error(), false)
	}

	// Open remote file
	remoteFile, err := s.client.Open(resolvedRemote)
	if err != nil {
		return NewToolError(CodeSFTPError, fmt.Sprintf("open remote file: %v", err), true)
	}
	defer remoteFile.Close()

	// Download to temp file
	tempPath := resolvedLocal + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return NewToolError(CodeLocalFileWriteFailed, err.Error(), false)
	}

	_, err = io.Copy(tempFile, remoteFile)
	tempFile.Close()
	if err != nil {
		os.Remove(tempPath)
		return NewToolError(CodeSFTPError, fmt.Sprintf("download data: %v", err), true)
	}

	// Atomic rename
	if err := os.Rename(tempPath, resolvedLocal); err != nil {
		os.Remove(tempPath)
		return NewToolError(CodeLocalFileWriteFailed, fmt.Sprintf("rename temp file: %v", err), false)
	}

	return nil
}

// ReadDir lists the contents of a remote directory.
func (s *SFTPClient) ReadDir(ctx context.Context, remotePath string) ([]os.FileInfo, error) {
	resolved, err := paths.ValidateRemote(remotePath, s.remoteRoots)
	if err != nil {
		return nil, NewToolError(CodeRemotePathNotAllowed, err.Error(), false)
	}

	entries, err := s.client.ReadDir(resolved)
	if err != nil {
		return nil, NewToolError(CodeSFTPError, fmt.Sprintf("read directory: %v", err), true)
	}
	return entries, nil
}

// DirSync recursively syncs a directory between local and remote.
func (s *SFTPClient) DirSync(ctx context.Context, direction, localPath, remotePath string) error {
	switch direction {
	case "upload":
		return s.dirSyncUpload(ctx, localPath, remotePath)
	case "download":
		return s.dirSyncDownload(ctx, localPath, remotePath)
	default:
		return NewToolError(CodeCommandValidationFailed,
			fmt.Sprintf("invalid direction %q, must be 'upload' or 'download'", direction), false)
	}
}

// dirSyncUpload recursively uploads a local directory to the remote server.
func (s *SFTPClient) dirSyncUpload(ctx context.Context, localPath, remotePath string) error {
	resolvedLocal, err := paths.ValidateLocal(localPath, s.localRoots, "read")
	if err != nil {
		return NewToolError(CodeLocalPathNotAllowed, err.Error(), false)
	}
	resolvedRemote, err := paths.ValidateRemote(remotePath, s.remoteRoots)
	if err != nil {
		return NewToolError(CodeRemotePathNotAllowed, err.Error(), false)
	}

	return filepath.Walk(resolvedLocal, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(resolvedLocal, p)
		rel = filepath.ToSlash(rel)
		remoteFile := path.Join(resolvedRemote, rel)

		return s.Upload(ctx, p, remoteFile)
	})
}

// dirSyncDownload recursively downloads a remote directory to the local machine.
func (s *SFTPClient) dirSyncDownload(ctx context.Context, localPath, remotePath string) error {
	resolvedLocal, err := paths.ValidateLocal(localPath, s.localRoots, "write")
	if err != nil {
		return NewToolError(CodeLocalPathNotAllowed, err.Error(), false)
	}
	resolvedRemote, err := paths.ValidateRemote(remotePath, s.remoteRoots)
	if err != nil {
		return NewToolError(CodeRemotePathNotAllowed, err.Error(), false)
	}

	walker := s.client.Walk(resolvedRemote)
	for walker.Step() {
		if walker.Err() != nil {
			return NewToolError(CodeSFTPError, walker.Err().Error(), true)
		}
		info := walker.Stat()
		if info.IsDir() {
			continue
		}

		rel := strings.TrimPrefix(walker.Path(), resolvedRemote)
		rel = strings.TrimPrefix(rel, "/")
		localFile := filepath.Join(resolvedLocal, filepath.FromSlash(rel))

		// Ensure local directory exists
		localDir := filepath.Dir(localFile)
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return NewToolError(CodeLocalFileWriteFailed, err.Error(), false)
		}

		if err := s.Download(ctx, walker.Path(), localFile); err != nil {
			return err
		}
	}
	return nil
}

// mkdirAll creates remote directories recursively.
func (s *SFTPClient) mkdirAll(remoteDir string) error {
	parts := strings.Split(path.Clean(remoteDir), "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			current = "/"
			continue
		}
		current = path.Join(current, part)
		if current == "/" {
			continue
		}
		if err := s.client.Mkdir(current); err != nil && !isSFTPAlreadyExists(err) {
			// Check if it's because the directory already exists
			if info, statErr := s.client.Stat(current); statErr == nil && info.IsDir() {
				continue
			}
			return err
		}
	}
	return nil
}

// isSFTPAlreadyExists checks if an SFTP error indicates the path already exists.
func isSFTPAlreadyExists(err error) bool {
	return strings.Contains(err.Error(), "already exists") ||
		strings.Contains(err.Error(), "Failure")
}

// SFTPTimeout applies the configured SFTP timeout to the context.
func SFTPTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return context.WithTimeout(ctx, timeout)
}
