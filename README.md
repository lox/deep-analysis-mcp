# GPT-5-Pro MCP

An MCP (Model Context Protocol) server that provides access to OpenAI's GPT-5-Pro for solving complex problems. GPT-5-Pro can read files and search codebases to gather context for analysis.

## Features

- **GPT-5-Pro Access**: Uses OpenAI's GPT-5-Pro model via the Responses API
- **File Operations**: GPT-5-Pro can read files and search with grep to gather information
- **Automatic Conversation Continuity**: Server-side conversation state via response IDs
- **Comprehensive Logging**: Stderr logging for debugging and monitoring

## Prerequisites

- Go 1.25.1 or later
- [OpenAI API Key](https://platform.openai.com/) with access to GPT-5-Pro
- [Hermit](https://cashapp.github.io/hermit/) (optional, for environment management)

## Installation

### Using Hermit (Recommended)

```bash
# Activate hermit environment
. bin/activate-hermit

# Install dependencies
go mod download

# Build
task build
```

### Without Hermit

```bash
# Install dependencies
go mod download

# Build
go build -o dist/gpt-5-pro-mcp .
```

## Configuration

Set your OpenAI API key as an environment variable:

```bash
export OPENAI_API_KEY="your-api-key-here"
```

Or use direnv with `.envrc`:

```bash
export OPENAI_API_KEY="your-api-key-here"
```

## Usage

### Install to Claude Code

```bash
task install:claude-code
```

This installs the MCP server to your user-level Claude Code configuration with the `OPENAI_API_KEY` from your environment.

### Manual Installation

Add to your MCP client configuration (e.g., `~/.claude.json`):

```json
{
  "mcpServers": {
    "gpt-5-pro": {
      "command": "/path/to/gpt-5-pro-mcp/dist/gpt-5-pro-mcp",
      "env": {
        "OPENAI_API_KEY": "your-api-key-here"
      }
    }
  }
}
```

### Testing

Test the server directly using mcp-tester:

```bash
mcp-tester call --tool=gpt-5-pro --json='{"prompt":"What is 2+2?"}' dist/gpt-5-pro-mcp
```

## The `gpt-5-pro` Tool

### Parameters

- **prompt** (required): The question or problem to analyze
- **continue** (optional, default: `true`): Continue previous conversation or start fresh

### Available Tools for GPT-5-Pro

GPT-5-Pro has access to these tools to gather information:

- **read_file**: Read contents of any file from the filesystem
- **grep_files**: Search for regex patterns in files matching glob patterns

GPT-5-Pro will automatically use these tools when it needs to examine code or gather context.

### Conversation Flow

Conversation state is managed server-side using OpenAI's Responses API:

- **continue: true** (default) - Continues from the previous response ID
- **continue: false** - Starts a fresh conversation

Conversation history persists for the lifetime of the MCP server process.

### Examples

**Single Query:**
```json
{
  "prompt": "Explain the performance implications of using channels vs mutexes in Go"
}
```

**Multi-Turn Conversation:**

Query 1:
```json
{
  "prompt": "I'm investigating a memory leak in a Go application. The heap grows continuously."
}
```

Query 2:
```json
{
  "prompt": "pprof shows 10,000+ goroutines blocked on channel receive.",
  "continue": true
}
```

Query 3 (continue defaults to true):
```json
{
  "prompt": "The work channel is never closed. How should I fix this?"
}
```

**Starting Fresh:**
```json
{
  "prompt": "New question about database indexing strategies",
  "continue": false
}
```

**With File Access:**

GPT-5-Pro will automatically read files when needed:

```json
{
  "prompt": "Review the error handling in internal/client/gpt5pro.go and suggest improvements"
}
```

GPT-5-Pro will use `read_file` to examine the code and provide specific recommendations.

## How It Works

This MCP uses OpenAI's Responses API with GPT-5-Pro. The system prompt guides the model to:

- Break down complex problems systematically
- Question assumptions and consider multiple perspectives
- Use file operations to gather evidence when analyzing code
- Provide clear, actionable insights
- Acknowledge uncertainty when appropriate

GPT-5-Pro can proactively read files and search codebases using its built-in tools without requiring explicit user requests.

## Development

```bash
# Build
task build

# Run tests
task test

# Run linter
task lint

# Clean build artifacts
task clean

# Tidy dependencies
task tidy
```

## Architecture

```
.
├── main.go                      # MCP server initialization
├── internal/
│   ├── client/
│   │   └── gpt5pro.go          # OpenAI Responses API client
│   ├── server/
│   │   └── mcp.go              # MCP server setup and tool registration
│   └── fileops/
│       └── fileops.go          # File operation handlers (read, grep)
└── Taskfile.yaml               # Build and development tasks
```

## Model Information

- **Model**: `gpt-5-pro`
- **Provider**: OpenAI
- **API**: OpenAI Responses API
- **Capabilities**: Advanced reasoning, function calling, extended context

## Logging

The server logs to stderr with timestamps. Logs include:

- Request details (prompt length, continue flag)
- API calls and responses
- Tool executions (file reads, grep operations)
- Response processing details

View logs when testing with mcp-tester or check Claude Code logs at `~/Library/Logs/Claude/`.

## Pricing

Pricing is determined by OpenAI. Check current rates at https://platform.openai.com/docs/models/gpt-5-pro

## License

MIT

## Contributing

Issues and pull requests welcome.
