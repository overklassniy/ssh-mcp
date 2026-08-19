package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/overklassniy/ssh-mcp/internal/ssh"
	"github.com/overklassniy/ssh-mcp/internal/status"
)

func registerServerStatus(s *server.MCPServer, manager *ssh.ConnectionManager) {
	s.AddTool(
		mcp.NewTool("server-status",
			mcp.WithDescription("Collect system status (hostname, OS, CPU, memory, disk, GPUs, processes, services) from a remote server via a single batched SSH command."),
			commonConnectionNameArg(),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			connName := req.GetString("connectionName", "")

			client, err := manager.GetClient(ctx, connName)
			if err != nil {
				return errorResult(err), nil
			}

			pol := manager.Policy(connName)

			runner := func(ctx context.Context, command, connName string) (string, error) {
				result, err := ssh.ExecCommandBypass(ctx, client, command, "", 0)
				if err != nil {
					return "", err
				}
				return result.Stdout, nil
			}

			statusResult, err := status.CollectSystemStatus(ctx, runner, connName, pol)
			if err != nil {
				return errorResult(err), nil
			}

			jsonBytes, _ := json.Marshal(statusResult)
			return mcp.NewToolResultText(string(jsonBytes)), nil
		},
	)
}
