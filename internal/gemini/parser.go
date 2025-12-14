package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ExtractCodeFromResponse extracts Go code blocks from a Gemini API response text.
// It looks for markdown code blocks with 'go' or 'golang' language identifier.
// Returns the extracted Go code or an error if no Go code block is found.
func ExtractCodeFromResponse(responseText string) (string, error) {
	// Pattern to match markdown code blocks with language identifier
	// Captures: ```language\ncode\n```
	pattern := regexp.MustCompile("(?s)```(\\w+)?\\n(.*?)```")
	allMatches := pattern.FindAllStringSubmatch(responseText, -1)

	if len(allMatches) == 0 {
		return "", fmt.Errorf("no code block found in response")
	}

	// Look for Go code blocks specifically
	for _, matches := range allMatches {
		if len(matches) >= 3 {
			language := strings.ToLower(matches[1])
			code := strings.TrimSpace(matches[2])

			// Check if it's a Go code block
			if (language == "go" || language == "golang") && code != "" {
				return code, nil
			}
		}
	}

	return "", fmt.Errorf("no Go code block found in response")
}

// WriteCodeToFile extracts Go code from a ChatHandler response and writes it to a .go file.
// If outputPath is empty, it generates a default filename "output.go".
// If outputPath doesn't have .go extension, it will be appended.
func WriteCodeToFile(response Response, outputPath string) (string, error) {
	code, err := ExtractCodeFromResponse(response.Text)
	if err != nil {
		return "", fmt.Errorf("failed to extract Go code: %w", err)
	}

	// Generate output path if not provided
	if outputPath == "" {
		outputPath = "output.go"
	} else if filepath.Ext(outputPath) != ".go" {
		// Ensure .go extension
		outputPath += ".go"
	}

	// Ensure the directory exists
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Write the code to the file
	if err := os.WriteFile(outputPath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return outputPath, nil
}

// ExtractAllGoCodeBlocks extracts all Go code blocks from a response text.
// Returns a slice of Go code strings.
func ExtractAllGoCodeBlocks(responseText string) ([]string, error) {
	pattern := regexp.MustCompile("(?s)```(\\w+)?\\n(.*?)```")
	matches := pattern.FindAllStringSubmatch(responseText, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no code blocks found in response")
	}

	var goBlocks []string
	for _, match := range matches {
		if len(match) >= 3 {
			language := strings.ToLower(match[1])
			code := strings.TrimSpace(match[2])

			// Only include Go code blocks
			if (language == "go" || language == "golang") && code != "" {
				goBlocks = append(goBlocks, code)
			}
		}
	}

	if len(goBlocks) == 0 {
		return nil, fmt.Errorf("no Go code blocks found")
	}

	return goBlocks, nil
}

// WriteAllGoCodeBlocks extracts all Go code blocks and writes them to separate .go files.
// Files are named with the pattern: baseFilename_1.go, baseFilename_2.go, etc.
// Returns the paths of all created files.
func WriteAllGoCodeBlocks(response Response, baseFilename string) ([]string, error) {
	blocks, err := ExtractAllGoCodeBlocks(response.Text)
	if err != nil {
		return nil, err
	}

	var filePaths []string
	for i, code := range blocks {
		filename := fmt.Sprintf("%s_%d.go", baseFilename, i+1)

		// Ensure the directory exists
		dir := filepath.Dir(filename)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return filePaths, fmt.Errorf("failed to create directory for %s: %w", filename, err)
			}
		}

		if err := os.WriteFile(filename, []byte(code), 0644); err != nil {
			return filePaths, fmt.Errorf("failed to write file %s: %w", filename, err)
		}

		filePaths = append(filePaths, filename)
	}

	return filePaths, nil
}
