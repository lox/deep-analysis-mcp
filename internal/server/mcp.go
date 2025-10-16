package server

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolHandler defines the interface for handling tool requests
type ToolHandler interface {
	Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// New creates and configures a new MCP server with the GPT-5-Pro tool
func New(handler ToolHandler) *server.MCPServer {
	s := server.NewMCPServer(
		"GPT-5-Pro MCP",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	gpt5ProTool := mcp.NewTool("gpt-5-pro",
		mcp.WithDescription("Consult GPT-5-Pro for complex problems requiring deep reasoning. GPT-5-Pro has access to read files and search file contents."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("The question or problem to analyze"),
		),
		mcp.WithBoolean("continue",
			mcp.Description("Continue previous conversation (true) or start fresh (false). Default: true"),
		),
	)

	s.AddTool(gpt5ProTool, handler.Handle)

	return s
}
