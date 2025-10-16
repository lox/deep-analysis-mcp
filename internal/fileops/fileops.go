package fileops

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Handler provides file operation capabilities
type Handler struct{}

// New creates a new file operations handler
func New() *Handler {
	return &Handler{}
}

// ReadFile reads a file and returns its contents
func (h *Handler) ReadFile(ctx context.Context, path string) (string, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// GrepFiles searches for a pattern in files
func (h *Handler) GrepFiles(ctx context.Context, pattern, pathPattern string, ignoreCase bool) (string, error) {
	// Compile regex
	flags := ""
	if ignoreCase {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Expand ~ to home directory
	if strings.HasPrefix(pathPattern, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		pathPattern = filepath.Join(home, pathPattern[1:])
	}

	// Find matching files
	matches, err := filepath.Glob(pathPattern)
	if err != nil {
		return "", fmt.Errorf("invalid path pattern: %w", err)
	}

	if len(matches) == 0 {
		return "No files matched the pattern", nil
	}

	var results []string

	// Search each file
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		file, err := os.Open(path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		var fileResults []string

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				fileResults = append(fileResults, fmt.Sprintf("%d:%s", lineNum, line))
			}
		}

		file.Close()

		if len(fileResults) > 0 {
			results = append(results, fmt.Sprintf("\n%s:", path))
			results = append(results, fileResults...)
		}
	}

	if len(results) == 0 {
		return "No matches found", nil
	}

	return strings.Join(results, "\n"), nil
}
