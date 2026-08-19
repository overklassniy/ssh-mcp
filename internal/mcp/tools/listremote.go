package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/overklassniy/ssh-mcp/internal/ssh"
)

func registerListRemote(s *server.MCPServer, manager *ssh.ConnectionManager) {
	s.AddTool(
		mcp.NewTool("list-remote",
			mcp.WithDescription("List the contents of a remote directory via SFTP. Returns structured file entries with name, size, mode, modTime, and isDir."),
			mcp.WithString("remotePath",
				mcp.Required(),
				mcp.Description("Remote directory path (absolute POSIX path)"),
			),
			commonConnectionNameArg(),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			remotePath, err := req.RequireString("remotePath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			connName := req.GetString("connectionName", "")

			client, err := manager.GetClient(ctx, connName)
			if err != nil {
				return errorResult(err), nil
			}

			srv := client.Config()
			if srv.Transport == "shell" {
				return errorResult(ssh.NewToolError(
					ssh.CodeUnsupportedInShellMode,
					"SFTP list-remote is not supported in shell transport mode", false)), nil
			}

			sftpCtx, cancel := ssh.SFTPTimeout(ctx, srv.GetSFTPTimeout())
			defer cancel()

			sftpClient, err := ssh.NewSFTPClient(client)
			if err != nil {
				return errorResult(err), nil
			}
			defer sftpClient.Close()

			entries, err := sftpClient.ReadDir(sftpCtx, remotePath)
			if err != nil {
				return errorResult(err), nil
			}

			files := make([]map[string]any, 0, len(entries))
			for _, entry := range entries {
				files = append(files, map[string]any{
					"name":    entry.Name(),
					"size":    entry.Size(),
					"mode":    fmt.Sprintf("%o", entry.Mode().Perm()),
					"modTime": entry.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
					"isDir":   entry.IsDir(),
				})
			}

			result := map[string]any{
				"path":  remotePath,
				"files": files,
				"count": len(files),
			}
			jsonBytes, _ := json.Marshal(result)
			return mcp.NewToolResultText(string(jsonBytes)), nil
		},
	)
}

// Ensure os import is used
var _ = os.Stdout
