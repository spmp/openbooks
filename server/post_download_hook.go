package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func parseDownloadMetadata(identifier string) downloadMetadata {
	identifier = strings.TrimSpace(identifier)
	identifier = strings.TrimPrefix(identifier, "!")

	if index := strings.Index(identifier, " ::INFO:: "); index != -1 {
		identifier = identifier[:index]
	}

	parts := strings.SplitN(identifier, " - ", 3)
	if len(parts) != 3 {
		return downloadMetadata{}
	}

	title := strings.TrimSpace(parts[2])
	ext := filepath.Ext(title)
	title = strings.TrimSuffix(title, ext)

	return downloadMetadata{
		Author: strings.TrimSpace(parts[1]),
		Title:  title,
	}
}

func (c *Client) runPostDownloadHook(scriptPath string, timeout time.Duration, filePath string, metadata downloadMetadata) {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	command := exec.CommandContext(ctx, scriptPath, filePath)
	command.Env = append(os.Environ(),
		fmt.Sprintf("OPENBOOKS_FILE_PATH=%s", filePath),
		fmt.Sprintf("OPENBOOKS_FILENAME=%s", filepath.Base(filePath)),
		fmt.Sprintf("OPENBOOKS_AUTHOR=%s", metadata.Author),
		fmt.Sprintf("OPENBOOKS_TITLE=%s", metadata.Title),
	)

	output, err := command.CombinedOutput()
	trimmedOutput := strings.TrimSpace(string(output))

	if ctx.Err() == context.DeadlineExceeded {
		c.log.Printf("post-download-hook timed out after %s: %s", timeout, scriptPath)
		if trimmedOutput != "" {
			c.log.Printf("post-download-hook output: %s", trimmedOutput)
		}
		return
	}

	if err != nil {
		c.log.Printf("post-download-hook failed: %v", err)
		if trimmedOutput != "" {
			c.log.Printf("post-download-hook output: %s", trimmedOutput)
		}
		return
	}

	if trimmedOutput != "" {
		c.log.Printf("post-download-hook output: %s", trimmedOutput)
	}
}
