package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/overklassniy/ssh-mcp/internal/ssh"
)

var (
	forwardManagers   = make(map[string]*ssh.ForwardManager)
	forwardManagersMu sync.Mutex
)

func getForwardManager(connName string, manager *ssh.ConnectionManager) *ssh.ForwardManager {
	forwardManagersMu.Lock()
	defer forwardManagersMu.Unlock()
	if fm, ok := forwardManagers[connName]; ok {
		return fm
	}
	fm := ssh.NewForwardManager()
	forwardManagers[connName] = fm
	return fm
}

func registerPortForward(s *server.MCPServer, manager *ssh.ConnectionManager) {
	s.AddTool(
		mcp.NewTool("port-forward",
			mcp.WithDescription("Manage SSH port forwards (local or remote). Supports open, close, and list actions."),
			mcp.WithString("action",
				mcp.Required(),
				mcp.Description("Action to perform: 'open', 'close', or 'list'"),
				mcp.Enum("open", "close", "list"),
			),
			mcp.WithString("direction",
				mcp.Description("Forward direction: 'local' or 'remote'. Required for 'open' action."),
				mcp.Enum("local", "remote"),
			),
			mcp.WithString("localAddr",
				mcp.Description("Local address (e.g. 'localhost:8080' or ':8080'). Required for 'open' action."),
			),
			mcp.WithString("remoteAddr",
				mcp.Description("Remote address (e.g. 'localhost:3000' or ':3000'). Required for 'open' action."),
			),
			mcp.WithString("id",
				mcp.Description("Forward ID. Required for 'close' action."),
			),
			commonConnectionNameArg(),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			action, err := req.RequireString("action")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			connName := req.GetString("connectionName", "")

			fm := getForwardManager(connName, manager)

			switch action {
			case "list":
				forwards := fm.ListForwards()
				result := map[string]any{
					"forwards": forwards,
					"count":    len(forwards),
				}
				jsonBytes, _ := json.Marshal(result)
				return mcp.NewToolResultText(string(jsonBytes)), nil

			case "open":
				direction, err := req.RequireString("direction")
				if err != nil {
					return mcp.NewToolResultError("direction is required for 'open' action"), nil
				}
				localAddr, err := req.RequireString("localAddr")
				if err != nil {
					return mcp.NewToolResultError("localAddr is required for 'open' action"), nil
				}
				remoteAddr, err := req.RequireString("remoteAddr")
				if err != nil {
					return mcp.NewToolResultError("remoteAddr is required for 'open' action"), nil
				}

				client, err := manager.GetClient(ctx, connName)
				if err != nil {
					return errorResult(err), nil
				}

				var entry *ssh.ForwardEntry
				switch direction {
				case "local":
					entry, err = fm.OpenLocalForward(ctx, client, localAddr, remoteAddr)
				case "remote":
					entry, err = fm.OpenRemoteForward(ctx, client, localAddr, remoteAddr)
				default:
					return mcp.NewToolResultError(fmt.Sprintf("invalid direction %q", direction)), nil
				}
				if err != nil {
					return errorResult(err), nil
				}

				result := map[string]any{
					"success":    true,
					"id":         entry.ID,
					"direction":  entry.Direction,
					"localAddr":  entry.LocalAddr,
					"remoteAddr": entry.RemoteAddr,
				}
				jsonBytes, _ := json.Marshal(result)
				return mcp.NewToolResultText(string(jsonBytes)), nil

			case "close":
				id, err := req.RequireString("id")
				if err != nil {
					return mcp.NewToolResultError("id is required for 'close' action"), nil
				}
				if err := fm.CloseForward(id); err != nil {
					return errorResult(err), nil
				}
				result := map[string]any{
					"success": true,
					"id":      id,
				}
				jsonBytes, _ := json.Marshal(result)
				return mcp.NewToolResultText(string(jsonBytes)), nil

			default:
				return mcp.NewToolResultError(fmt.Sprintf("invalid action %q", action)), nil
			}
		},
	)
}
