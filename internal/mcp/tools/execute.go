package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/overklassniy/ssh-mcp/internal/ssh"
)

func registerExecuteCommand(s *server.MCPServer, manager *ssh.ConnectionManager) {
	s.AddTool(
		mcp.NewTool("execute-command",
			mcp.WithDescription("Execute a command on a remote server via SSH. Returns stdout, stderr, exit code, and duration."),
			mcp.WithString("cmdString",
				mcp.Required(),
				mcp.Description("The command to execute"),
			),
			mcp.WithString("directory",
				mcp.Description("Working directory for the command (optional)"),
			),
			mcp.WithString("timeout",
				mcp.Description("Per-call timeout as a duration string (e.g. \"30s\", \"5m\"). Defaults to the server's command_timeout."),
			),
			commonConnectionNameArg(),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cmdString, err := req.RequireString("cmdString")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			directory := req.GetString("directory", "")
			timeoutStr := req.GetString("timeout", "")
			connName := req.GetString("connectionName", "")

			var timeout time.Duration
			if timeoutStr != "" {
				timeout, err = time.ParseDuration(timeoutStr)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid timeout: %v", err)), nil
				}
			}

			client, err := manager.GetClient(ctx, connName)
			if err != nil {
				return errorResult(err), nil
			}

			srv := client.Config()

			var result *ssh.ExecResult
			if srv.Transport == "shell" {
				runner := ssh.NewShellRunner(client)
				defer runner.Close()
				result, err = runner.ExecCommand(ctx, cmdString, directory, timeout)
			} else {
				result, err = ssh.ExecCommand(ctx, client, cmdString, directory, timeout)
			}
			if err != nil {
				return errorResult(err), nil
			}

			jsonBytes, _ := json.Marshal(result)
			return mcp.NewToolResultText(string(jsonBytes)), nil
		},
	)
}
