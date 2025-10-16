package main

import (
	"log"
	"os"

	"github.com/lox/gpt-5-pro-mcp/internal/client"
	"github.com/lox/gpt-5-pro-mcp/internal/fileops"
	"github.com/lox/gpt-5-pro-mcp/internal/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	f := fileops.New()
	c := client.New(apiKey, f)
	s := server.New(c)

	if err := mcpserver.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
