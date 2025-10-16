package main

import (
	"flag"
	"log"
	"os"

	"github.com/lox/deep-analysis-mcp/internal/client"
	"github.com/lox/deep-analysis-mcp/internal/fileops"
	"github.com/lox/deep-analysis-mcp/internal/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func main() {
	// Configure logging to stderr
	log.SetOutput(os.Stderr)
	log.SetPrefix("[deep-analysis-mcp] ")
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	// CLI flags
	transport := flag.String("transport", "stdio", "Transport type: stdio, sse, or http")
	addr := flag.String("addr", ":8080", "Address to listen on for HTTP/SSE transports")
	flag.Parse()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	f := fileops.New()
	c := client.New(apiKey, f)
	s := server.New(c)

	switch *transport {
	case "stdio":
		log.Println("Starting MCP server with stdio transport")
		if err := mcpserver.ServeStdio(s); err != nil {
			log.Fatal(err)
		}

	case "sse":
		log.Printf("Starting MCP server with SSE transport on %s", *addr)
		sseServer := mcpserver.NewSSEServer(s,
			mcpserver.WithBasePath("/sse"),
		)
		if err := sseServer.Start(*addr); err != nil {
			log.Fatal(err)
		}

	case "http":
		log.Printf("Starting MCP server with HTTP streaming transport on %s", *addr)
		httpServer := mcpserver.NewStreamableHTTPServer(s)
		if err := httpServer.Start(*addr); err != nil {
			log.Fatal(err)
		}

	default:
		log.Fatalf("Unknown transport: %s (must be stdio, sse, or http)", *transport)
	}
}
