// Package tools implements the MCP tools exposed by ssh-mcp.
package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/overklassniy/ssh-mcp/internal/ssh"
)

// RegisterAll registers all 8 tools on the given MCP server.
func RegisterAll(s *server.MCPServer, manager *ssh.ConnectionManager) {
	registerExecuteCommand(s, manager)
	registerUpload(s, manager)
	registerDownload(s, manager)
	registerListServers(s, manager)
	registerServerStatus(s, manager)
	registerListRemote(s, manager)
	registerDirSync(s, manager)
	registerPortForward(s, manager)
}

// commonConnectionNameArg is the optional connectionName argument
// shared by all tools.
func commonConnectionNameArg() mcp.ToolOption {
	return mcp.WithString("connectionName",
		mcp.Description("Name of the configured server to use. Defaults to the first server."),
	)
}

// errorResult creates a tool result from a ToolError, preserving the
// error code and retriable flag in structured content.
func errorResult(err error) *mcp.CallToolResult {
	if te, ok := err.(*ssh.ToolError); ok {
		return mcp.NewToolResultError(te.Error())
	}
	return mcp.NewToolResultError(err.Error())
}
