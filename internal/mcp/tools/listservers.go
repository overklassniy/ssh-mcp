package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/overklassniy/ssh-mcp/internal/ssh"
)

func registerListServers(s *server.MCPServer, manager *ssh.ConnectionManager) {
	s.AddTool(
		mcp.NewTool("list-servers",
			mcp.WithDescription("List all configured SSH servers with their connection status."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cfg := manager.Config()
			servers := make([]map[string]any, 0, len(cfg.Servers))
			for _, srv := range cfg.Servers {
				entry := map[string]any{
					"name":     srv.Name,
					"host":     srv.Host,
					"port":     srv.Port,
					"username": srv.Username,
					"transport": srv.Transport,
				}
				// Check connection status
				client, err := manager.GetClient(ctx, srv.Name)
				if err == nil && client.IsConnected() {
					entry["connected"] = true
				} else {
					entry["connected"] = false
				}
				servers = append(servers, entry)
			}

			result := map[string]any{
				"servers": servers,
				"count":   len(servers),
			}
			jsonBytes, _ := json.Marshal(result)
			return mcp.NewToolResultText(string(jsonBytes)), nil
		},
	)
}
