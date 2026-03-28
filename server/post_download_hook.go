package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var runHookCommand = executeHookCommand

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

	c.log.Printf("post-download-hook triggered: command=%q arg=%q", scriptPath, filePath)

	if c.hookWorkerLimiter != nil {
		c.hookWorkerLimiter <- struct{}{}
		defer func() {
			<-c.hookWorkerLimiter
		}()
	}

	output, timedOut, err := runHookCommand(scriptPath, timeout, filePath, metadata)
	trimmedOutput := strings.TrimSpace(string(output))

	if timedOut {
		c.log.Printf("post-download-hook timed out after %s: %s", timeout, scriptPath)
		c.logHookScriptDetails(scriptPath)
		if trimmedOutput != "" {
			c.log.Printf("post-download-hook output: %s", trimmedOutput)
		}
		return
	}

	if err != nil {
		c.log.Printf("post-download-hook failed: script=%q file=%q err=%v", scriptPath, filePath, err)
		c.logHookExecutionError(err)
		c.logHookScriptDetails(scriptPath)
		if trimmedOutput != "" {
			c.log.Printf("post-download-hook output: %s", trimmedOutput)
		}
		return
	}

	if trimmedOutput != "" {
		c.log.Printf("post-download-hook output: %s", trimmedOutput)
	}
}

func executeHookCommand(scriptPath string, timeout time.Duration, filePath string, metadata downloadMetadata) ([]byte, bool, error) {
	cmd := exec.Command(scriptPath, filePath)
	configureHookProcess(cmd)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("OPENBOOKS_FILE_PATH=%s", filePath),
		fmt.Sprintf("OPENBOOKS_FILENAME=%s", filepath.Base(filePath)),
		fmt.Sprintf("OPENBOOKS_AUTHOR=%s", metadata.Author),
		fmt.Sprintf("OPENBOOKS_TITLE=%s", metadata.Title),
	)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return append(stdout.Bytes(), stderr.Bytes()...), false, err
	}

	waitComplete := make(chan error, 1)
	go func() {
		waitComplete <- cmd.Wait()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-waitComplete:
		return append(stdout.Bytes(), stderr.Bytes()...), false, err
	case <-timer.C:
		killHookProcess(cmd)
		waitErr := <-waitComplete
		if waitErr != nil && !errors.Is(waitErr, context.DeadlineExceeded) {
			return append(stdout.Bytes(), stderr.Bytes()...), true, waitErr
		}
		return append(stdout.Bytes(), stderr.Bytes()...), true, context.DeadlineExceeded
	}
}

func (c *Client) logHookScriptDetails(scriptPath string) {
	info, err := os.Stat(scriptPath)
	if err != nil {
		c.log.Printf("post-download-hook stat failed: path=%q err=%v", scriptPath, err)
		return
	}

	c.log.Printf("post-download-hook script info: path=%q mode=%s", scriptPath, info.Mode())

	file, err := os.Open(scriptPath)
	if err != nil {
		c.log.Printf("post-download-hook unable to inspect shebang: path=%q err=%v", scriptPath, err)
		return
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		c.log.Printf("post-download-hook unable to read script header: path=%q err=%v", scriptPath, err)
		return
	}

	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "#!") {
		shebang := strings.TrimPrefix(line, "#!")
		parts := strings.Fields(shebang)
		if len(parts) > 0 {
			if _, err := os.Stat(parts[0]); err != nil {
				c.log.Printf("post-download-hook shebang interpreter missing: interpreter=%q err=%v", parts[0], err)
			} else {
				c.log.Printf("post-download-hook shebang interpreter found: interpreter=%q", parts[0])
			}
		}
	}
}

func (c *Client) logHookExecutionError(err error) {
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		c.log.Printf("post-download-hook exec error: name=%q err=%v", execErr.Name, execErr.Err)
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		c.log.Printf("post-download-hook path error: op=%q path=%q err=%v", pathErr.Op, pathErr.Path, pathErr.Err)
	}
}
