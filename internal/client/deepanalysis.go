package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
)

const (
	defaultModel  = "gpt-5-pro"
	maxIterations = 10 // Limit function call iterations
)

// FileOps defines the interface for file operations
type FileOps interface {
	ReadFile(ctx context.Context, path string) (string, error)
	GrepFiles(ctx context.Context, pattern, path string, ignoreCase bool) (string, error)
	GlobFiles(ctx context.Context, pattern string) (string, error)
}

// DeepAnalysisClient handles communication with OpenAI's Responses API
type DeepAnalysisClient struct {
	client  *openai.Client
	fileOps FileOps
	conv    map[string]string // conversation_id -> response_id
	mu      sync.RWMutex
	tools   []responses.ToolUnionParam
}

// New creates a new DeepAnalysisClient instance
func New(apiKey string, fileOps FileOps) *DeepAnalysisClient {
	client := openai.NewClient(option.WithAPIKey(apiKey))

	c := &DeepAnalysisClient{
		client:  &client,
		fileOps: fileOps,
		conv:    make(map[string]string),
	}
	c.tools = c.buildTools()

	return c
}

// Handle processes a consultation request using Responses API
func (c *DeepAnalysisClient) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	task, err := request.RequireString("task")
	if err != nil {
		log.Printf("ERROR: Failed to get task: %v", err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	context := request.GetString("context", "")
	files := request.GetStringSlice("files", nil)
	continueConversation := request.GetBool("continue", true)
	conversationID := request.GetString("conversation_id", "")
	
	// Use default conversation ID if none provided
	if conversationID == "" {
		conversationID = "default"
	}
	
	// Read attached files if provided
	var filesContent string
	if len(files) > 0 {
		log.Printf("Reading %d attached files", len(files))
		var fileParts []string
		for _, filePath := range files {
			content, err := c.fileOps.ReadFile(ctx, filePath)
			if err != nil {
				log.Printf("WARNING: Failed to read file %s: %v", filePath, err)
				fileParts = append(fileParts, fmt.Sprintf("File: %s\nError: %v\n", filePath, err))
			} else {
				log.Printf("Successfully read file: %s (%d bytes)", filePath, len(content))
				fileParts = append(fileParts, fmt.Sprintf("File: %s\n```\n%s\n```\n", filePath, content))
			}
		}
		filesContent = "\n" + fmt.Sprintf("Attached Files:\n%s\n", joinStrings(fileParts, "\n"))
	}
	
	// Build the full prompt with context and files if provided
	var prompt string
	if context != "" && filesContent != "" {
		prompt = fmt.Sprintf("Context:\n%s%s\nTask:\n%s", context, filesContent, task)
	} else if context != "" {
		prompt = fmt.Sprintf("Context:\n%s\n\nTask:\n%s", context, task)
	} else if filesContent != "" {
		prompt = fmt.Sprintf("%s\nTask:\n%s", filesContent, task)
	} else {
		prompt = task
	}
	
	log.Printf("Received request: task_len=%d context_len=%d files=%d continue=%v conversation_id=%q", len(task), len(context), len(files), continueConversation, conversationID)

	// Get previous response ID if continuing
	var prevResponseID string
	if continueConversation {
		prevResponseID = c.getRespID(conversationID)
		if prevResponseID != "" {
			log.Printf("Continuing conversation: id=%s response_id=%s", conversationID, prevResponseID)
		} else {
			log.Printf("Starting fresh conversation: id=%s", conversationID)
		}
	} else {
		log.Printf("Starting fresh conversation (continue=false)")
		// Clear existing conversation state
		c.clearRespID(conversationID)
	}

	// Build the request parameters
	params := responses.ResponseNewParams{
		Model:        defaultModel,
		Instructions: openai.Opt(buildSystemPrompt()),
		Tools:        c.tools,
	}

	// Add input message
	inputItems := responses.ResponseInputParam{
		responses.ResponseInputItemParamOfMessage(prompt, responses.EasyInputMessageRoleUser),
	}
	params.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: inputItems,
	}

	// Add previous response ID if continuing
	if prevResponseID != "" {
		params.PreviousResponseID = openai.Opt(prevResponseID)
	}

	// Call OpenAI Responses API
	log.Printf("Calling OpenAI Responses API: model=%s", defaultModel)
	response, err := c.client.Responses.New(ctx, params)
	if err != nil {
		log.Printf("ERROR: OpenAI API call failed: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("OpenAI API error: %v", err)), nil
	}

	// Save the response ID for conversation continuity
	if conversationID != "" {
		c.setRespID(conversationID, response.ID)
	}
	log.Printf("Received response: id=%s status=%s", response.ID, response.Status)

	// Handle tool calls in a loop
	for i := 0; i < maxIterations; i++ {
		// Check if there are tool calls to execute
		toolCalls := extractToolCalls(response)
		log.Printf("Iteration %d: found %d tool calls", i+1, len(toolCalls))

		if len(toolCalls) == 0 {
			// No more tool calls, extract and return final text response
			text := extractTextContent(response)
			log.Printf("No tool calls, returning text response: len=%d", len(text))
			if text == "" {
				log.Printf("ERROR: No text content in response")
				return mcp.NewToolResultError("No text content in response"), nil
			}
			return mcp.NewToolResultText(text), nil
		}

		// Execute tool calls
		toolOutputs := make(responses.ResponseInputParam, 0, len(toolCalls))
		for _, toolCall := range toolCalls {
			log.Printf("Executing tool: name=%s id=%s args_len=%d", toolCall.Name, toolCall.ID, len(toolCall.Arguments))
			result, err := c.executeFunction(ctx, toolCall.Name, toolCall.Arguments)
			if err != nil {
				log.Printf("Tool execution error: %v", err)
				result = fmt.Sprintf("Error: %v", err)
			} else {
				log.Printf("Tool execution success: result_len=%d", len(result))
			}

			toolOutputs = append(toolOutputs, responses.ResponseInputItemParamOfFunctionCallOutput(toolCall.ID, result))
		}

		// Continue the response with tool outputs
		log.Printf("Continuing with %d tool outputs", len(toolOutputs))
		params = responses.ResponseNewParams{
			Model:              defaultModel,
			PreviousResponseID: openai.Opt(response.ID),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: toolOutputs,
			},
			Tools: c.tools,
		}

		response, err = c.client.Responses.New(ctx, params)
		if err != nil {
			log.Printf("ERROR: Follow-up API call failed: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("OpenAI API error: %v", err)), nil
		}

		// Update response ID
		if conversationID != "" {
			c.setRespID(conversationID, response.ID)
		}
		log.Printf("Updated response: id=%s status=%s", response.ID, response.Status)
	}

	log.Printf("ERROR: Max iterations (%d) reached", maxIterations)
	return mcp.NewToolResultError("Max function call iterations reached"), nil
}

// getRespID safely retrieves a response ID for a conversation
func (c *DeepAnalysisClient) getRespID(conversationID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conv[conversationID]
}

// setRespID safely stores a response ID for a conversation
func (c *DeepAnalysisClient) setRespID(conversationID, responseID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conv[conversationID] = responseID
}

// clearRespID safely clears a conversation's response ID
func (c *DeepAnalysisClient) clearRespID(conversationID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.conv, conversationID)
}

// buildTools defines the tools available to the model
func (c *DeepAnalysisClient) buildTools() []responses.ToolUnionParam {
	return []responses.ToolUnionParam{
		responses.ToolParamOfFunction(
			"read_file",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file to read (supports ~ for home directory)",
						"minLength":   1,
					},
				},
				"required":             []string{"path"},
				"additionalProperties": false,
			},
			true, // strict
		),
		responses.ToolParamOfFunction(
			"grep_files",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Regular expression pattern to search for",
						"minLength":   1,
					},
					"path": map[string]any{
						"type":        "string",
						"description": "File path or glob pattern (e.g., '*.go', 'src/*.js') using shell-style wildcards (* and ?)",
						"minLength":   1,
					},
					"ignore_case": map[string]any{
						"type":        "boolean",
						"description": "Perform case-insensitive search",
						"default":     false,
					},
				},
				"required":             []string{"pattern", "path", "ignore_case"},
				"additionalProperties": false,
			},
			true, // strict
		),
		responses.ToolParamOfFunction(
			"glob_files",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Glob pattern (e.g., '**/*.go', 'internal/**/test_*.go', '*.{js,ts}'). Use ** for recursive matching, * for files/dirs, ? for single char.",
						"minLength":   1,
					},
				},
				"required":             []string{"pattern"},
				"additionalProperties": false,
			},
			true, // strict
		),
	}
}

// executeFunction executes a function call requested by the model
func (c *DeepAnalysisClient) executeFunction(ctx context.Context, name, argsJSON string) (string, error) {
	switch name {
	case "read_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		return c.fileOps.ReadFile(ctx, args.Path)

	case "grep_files":
		var args struct {
			Pattern    string `json:"pattern"`
			Path       string `json:"path"`
			IgnoreCase bool   `json:"ignore_case"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		return c.fileOps.GrepFiles(ctx, args.Pattern, args.Path, args.IgnoreCase)

	case "glob_files":
		var args struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		return c.fileOps.GlobFiles(ctx, args.Pattern)

	default:
		return "", fmt.Errorf("unknown function: %s", name)
	}
}

// ToolCall represents a function tool call
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// extractToolCalls extracts tool calls from a response
func extractToolCalls(response *responses.Response) []ToolCall {
	var toolCalls []ToolCall

	log.Printf("Extracting tool calls from %d output items", len(response.Output))
	for i, item := range response.Output {
		log.Printf("Output item %d: type=%s", i, item.Type)
		if item.Type == "function_call" {
			toolCalls = append(toolCalls, ToolCall{
				ID:        item.CallID,
				Name:      item.Name,
				Arguments: item.Arguments,
			})
			log.Printf("Found function call: name=%s id=%s", item.Name, item.CallID)
		}
	}

	return toolCalls
}

// extractTextContent extracts text content from a response
func extractTextContent(response *responses.Response) string {
	var textParts []string

	log.Printf("Extracting text content from %d output items", len(response.Output))
	for i, item := range response.Output {
		log.Printf("Output item %d: type=%s content_items=%d", i, item.Type, len(item.Content))
		if item.Type == "message" {
			for j, contentItem := range item.Content {
				log.Printf("  Content item %d: type=%s", j, contentItem.Type)
				// The Responses API uses "output_text" not "text"
				if contentItem.Type == "text" || contentItem.Type == "output_text" {
					textParts = append(textParts, contentItem.Text)
					log.Printf("  Found text: len=%d", len(contentItem.Text))
				}
			}
		}
	}

	result := ""
	for _, part := range textParts {
		if result != "" {
			result += "\n"
		}
		result += part
	}

	log.Printf("Extracted %d text parts, total length=%d", len(textParts), len(result))
	return result
}

// joinStrings joins strings with a separator
func joinStrings(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}

// buildSystemPrompt creates the system prompt
func buildSystemPrompt() string {
	return `You are an expert deep analysis AI consulted for the most challenging and complex problems.

Your role is to provide deep, systematic analysis through multi-step reasoning:

1. **Context Gathering**: FIRST, proactively use your tools to gather relevant information:
   - Read files mentioned in the context or task
   - Search for related code, configuration, or documentation
   - Understand the full picture before forming conclusions
   
2. **Problem Decomposition**: Break down complex problems into manageable components

3. **Hypothesis Generation**: Form clear theories about root causes or solutions based on evidence

4. **Systematic Investigation**: Work through problems methodically, step by step

5. **Confidence Assessment**: Honestly evaluate certainty levels at each stage

6. **Actionable Recommendations**: Provide concrete, specific next steps

When analyzing problems:
- **Start by gathering context** - Read relevant files, search for patterns, understand the codebase
- Think deeply and systematically
- Question assumptions and verify them with evidence
- Consider multiple perspectives
- Identify gaps in understanding and fill them proactively
- Provide clear, actionable insights
- Acknowledge uncertainty when appropriate
- Suggest concrete next steps with examples

Your responses should be:
- **Evidence-based**: Always gather information with your tools before concluding
- **Thorough**: Cover all relevant aspects
- **Clear**: Easy to understand and act upon
- **Structured**: Organized logically
- **Actionable**: Include concrete recommendations with code examples when relevant

**Available Tools**:
You have access to the following tools to gather information:

1. **glob_files(pattern)**: Discover files matching a pattern
   - Examples: "**/*.go" (all Go files), "internal/**/test_*.go" (test files in internal), "*.{js,ts}" (JS/TS files)
   - Use this FIRST when you don't know exact file paths
   - Directories marked with trailing /

2. **read_file(path)**: Read the contents of any file
   - Use after discovering files with glob_files
   - Supports ~ for home directory

3. **grep_files(pattern, path, ignore_case)**: Search for regex patterns in files
   - pattern: Regular expression to search for
   - path: Glob pattern for files to search (e.g., "*.go", "src/**/*.js")
   - Use to find specific code patterns across multiple files

**Attached Files**:
Sometimes files will be pre-attached to your prompt under "Attached Files". Review these carefully as they contain the key code/config you need to analyze.

**CRITICAL WORKFLOW** - Use these tools PROACTIVELY and FREQUENTLY:
1. **Discover**: Use glob_files to find relevant files if you don't know exact paths
2. **Review**: Read any pre-attached files first
3. **Investigate**: Read additional files mentioned or discovered
4. **Search**: Use grep_files to find patterns or references across the codebase
5. **Verify**: Don't make assumptions - gather evidence before concluding

You are being consulted because standard approaches have proven insufficient. Bring your full analytical capabilities to bear, and let the evidence guide your recommendations.`
}
