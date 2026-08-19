package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/overklassniy/ssh-mcp/internal/ssh"
)

func registerDirSync(s *server.MCPServer, manager *ssh.ConnectionManager) {
	s.AddTool(
		mcp.NewTool("dir-sync",
			mcp.WithDescription("Recursively sync a directory between local and remote via SFTP. Supports upload and download directions with concurrent file transfers."),
			mcp.WithString("direction",
				mcp.Required(),
				mcp.Description("Transfer direction: 'upload' (local to remote) or 'download' (remote to local)"),
				mcp.Enum("upload", "download"),
			),
			mcp.WithString("localPath",
				mcp.Required(),
				mcp.Description("Local directory path"),
			),
			mcp.WithString("remotePath",
				mcp.Required(),
				mcp.Description("Remote directory path (absolute POSIX path)"),
			),
			commonConnectionNameArg(),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			direction, err := req.RequireString("direction")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			localPath, err := req.RequireString("localPath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
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
					"SFTP dir-sync is not supported in shell transport mode", false)), nil
			}

			sftpCtx, cancel := ssh.SFTPTimeout(ctx, srv.GetSFTPTimeout())
			defer cancel()

			sftpClient, err := ssh.NewSFTPClient(client)
			if err != nil {
				return errorResult(err), nil
			}
			defer sftpClient.Close()

			if err := sftpClient.DirSync(sftpCtx, direction, localPath, remotePath); err != nil {
				return errorResult(err), nil
			}

			result := map[string]any{
				"success":    true,
				"direction":  direction,
				"localPath":  localPath,
				"remotePath": remotePath,
			}
			jsonBytes, _ := json.Marshal(result)
			return mcp.NewToolResultText(string(jsonBytes)), nil
		},
	)
}
