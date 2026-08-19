package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/overklassniy/ssh-mcp/internal/ssh"
)

func registerDownload(s *server.MCPServer, manager *ssh.ConnectionManager) {
	s.AddTool(
		mcp.NewTool("download",
			mcp.WithDescription("Download a remote file to the local machine via SFTP. Downloads to a temp file first, then atomically renames. Validates paths."),
			mcp.WithString("remotePath",
				mcp.Required(),
				mcp.Description("Remote file path (absolute POSIX path)"),
			),
			mcp.WithString("localPath",
				mcp.Required(),
				mcp.Description("Local file path to download to"),
			),
			commonConnectionNameArg(),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			remotePath, err := req.RequireString("remotePath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			localPath, err := req.RequireString("localPath")
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
					"SFTP download is not supported in shell transport mode", false)), nil
			}

			sftpCtx, cancel := ssh.SFTPTimeout(ctx, srv.GetSFTPTimeout())
			defer cancel()

			sftpClient, err := ssh.NewSFTPClient(client)
			if err != nil {
				return errorResult(err), nil
			}
			defer sftpClient.Close()

			if err := sftpClient.Download(sftpCtx, remotePath, localPath); err != nil {
				return errorResult(err), nil
			}

			result := map[string]any{
				"success":    true,
				"remotePath": remotePath,
				"localPath":  localPath,
			}
			jsonBytes, _ := json.Marshal(result)
			return mcp.NewToolResultText(string(jsonBytes)), nil
		},
	)
}
