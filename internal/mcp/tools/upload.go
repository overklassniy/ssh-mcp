package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/overklassniy/ssh-mcp/internal/ssh"
)

func registerUpload(s *server.MCPServer, manager *ssh.ConnectionManager) {
	s.AddTool(
		mcp.NewTool("upload",
			mcp.WithDescription("Upload a local file to a remote server via SFTP. Validates local and remote paths against configured allowed paths."),
			mcp.WithString("localPath",
				mcp.Required(),
				mcp.Description("Local file path to upload"),
			),
			mcp.WithString("remotePath",
				mcp.Required(),
				mcp.Description("Remote file path (absolute POSIX path)"),
			),
			commonConnectionNameArg(),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
					"SFTP upload is not supported in shell transport mode", false)), nil
			}

			sftpCtx, cancel := ssh.SFTPTimeout(ctx, srv.GetSFTPTimeout())
			defer cancel()

			sftpClient, err := ssh.NewSFTPClient(client)
			if err != nil {
				return errorResult(err), nil
			}
			defer sftpClient.Close()

			if err := sftpClient.Upload(sftpCtx, localPath, remotePath); err != nil {
				return errorResult(err), nil
			}

			result := map[string]any{
				"success":    true,
				"localPath":  localPath,
				"remotePath": remotePath,
			}
			jsonBytes, _ := json.Marshal(result)
			return mcp.NewToolResultText(string(jsonBytes)), nil
		},
	)
}
