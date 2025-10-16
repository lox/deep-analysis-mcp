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

// New creates and configures a new MCP server with the deep-analysis tool
func New(handler ToolHandler) *server.MCPServer {
	s := server.NewMCPServer(
		"Deep Analysis MCP",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	deepAnalysisTool := mcp.NewTool("deep-analysis",
		mcp.WithDescription("Consult a deep analysis AI for complex problems requiring systematic reasoning. The AI has access to read files, search file contents, and discover files via glob patterns."),
		mcp.WithString("task",
			mcp.Required(),
			mcp.Description("The specific question or task you want analyzed. Be clear about what kind of analysis, review, or guidance you need."),
		),
		mcp.WithString("context",
			mcp.Description("Optional context about the current situation, what you've tried, background information, or relevant details that would help provide better guidance."),
		),
		mcp.WithArray("files",
			mcp.Description("Optional list of file paths to attach. These files will be automatically read and included in the analysis."),
			mcp.WithStringItems(),
		),
		mcp.WithString("conversation_id",
			mcp.Description("Identifier to continue a specific conversation; omit to start fresh"),
		),
		mcp.WithBoolean("continue",
			mcp.Description("Continue previous conversation (true) or start fresh (false). Default: true"),
		),
	)

	s.AddTool(deepAnalysisTool, handler.Handle)

	return s
}
