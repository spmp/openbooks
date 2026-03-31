package server

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestRunPostDownloadHookTimeoutKillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group reap test is unix-specific")
	}

	tempDir := t.TempDir()
	childPidPath := filepath.Join(tempDir, "child.pid")
	markerPath := filepath.Join(tempDir, "ok.txt")

	timeoutScript := filepath.Join(tempDir, "timeout.sh")
	timeoutScriptContent := "#!/usr/bin/env bash\n" +
		"sleep 30 &\n" +
		"child=$!\n" +
		"echo $child > \"" + childPidPath + "\"\n" +
		"wait $child\n"
	if err := os.WriteFile(timeoutScript, []byte(timeoutScriptContent), 0755); err != nil {
		t.Fatalf("write timeout script: %v", err)
	}

	successScript := filepath.Join(tempDir, "ok.sh")
	successScriptContent := "#!/usr/bin/env bash\n" +
		"echo ok > \"" + markerPath + "\"\n"
	if err := os.WriteFile(successScript, []byte(successScriptContent), 0755); err != nil {
		t.Fatalf("write success script: %v", err)
	}

	client := &Client{
		log:               log.New(io.Discard, "", 0),
		hookWorkerLimiter: make(chan struct{}, 1),
	}

	start := time.Now()
	client.runPostDownloadHook(timeoutScript, 200*time.Millisecond, filepath.Join(tempDir, "book.epub"), downloadMetadata{})
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("timeout hook took too long to return: %s", elapsed)
	}

	pidBytes, err := os.ReadFile(childPidPath)
	if err != nil {
		t.Fatalf("read child pid: %v", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Fatalf("parse child pid: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			t.Fatalf("unexpected child process state error: %v", err)
		}

		commandLine, readErr := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
		if readErr == nil && !strings.Contains(string(commandLine), "sleep") {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("expected child process %d to be reaped", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}

	client.runPostDownloadHook(successScript, 2*time.Second, filepath.Join(tempDir, "book.epub"), downloadMetadata{})
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("expected backend to continue and run next hook: %v", err)
	}
}

func TestRunPostDownloadHookQueuesByWorkerLimit(t *testing.T) {
	originalRunner := runHookCommand
	defer func() {
		runHookCommand = originalRunner
	}()

	var active int32
	var maxActive int32
	runHookCommand = func(_ string, _ time.Duration, _ string, _ downloadMetadata) ([]byte, bool, error) {
		current := atomic.AddInt32(&active, 1)
		for {
			observed := atomic.LoadInt32(&maxActive)
			if current <= observed {
				break
			}
			if atomic.CompareAndSwapInt32(&maxActive, observed, current) {
				break
			}
		}

		time.Sleep(150 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return nil, false, nil
	}

	client := &Client{
		log:               log.New(io.Discard, "", 0),
		hookWorkerLimiter: make(chan struct{}, 1),
	}

	start := time.Now()
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		client.runPostDownloadHook("ignored", time.Second, "/tmp/a", downloadMetadata{})
	}()
	go func() {
		defer waitGroup.Done()
		client.runPostDownloadHook("ignored", time.Second, "/tmp/b", downloadMetadata{})
	}()
	waitGroup.Wait()

	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("expected max concurrent hooks to be 1, got %d", got)
	}

	if elapsed := time.Since(start); elapsed < 250*time.Millisecond {
		t.Fatalf("expected queued hook execution, elapsed=%s", elapsed)
	}
}

func TestRunPostDownloadHookSendsNotificationsFromOutput(t *testing.T) {
	originalRunner := runHookCommand
	defer func() {
		runHookCommand = originalRunner
	}()

	runHookCommand = func(_ string, _ time.Duration, _ string, _ downloadMetadata) ([]byte, bool, error) {
		output := strings.Join([]string{
			"OPENBOOKS_NOTIFY {\"level\":\"info\",\"title\":\"Processed\",\"detail\":\"All good\"}",
			"OPENBOOKS_NOTIFY {\"level\":\"warning\",\"title\":\"Skipped\",\"detail\":\"Missing tag\"}",
			"OPENBOOKS_NOTIFY {\"level\":\"error\",\"title\":\"Failed\",\"detail\":\"Move failed\"}",
		}, "\n")
		return []byte(output), false, nil
	}

	client := &Client{
		log:               log.New(io.Discard, "", 0),
		hookWorkerLimiter: make(chan struct{}, 1),
		send:              make(chan interface{}, 8),
	}

	client.runPostDownloadHook("ignored", time.Second, "/tmp/book.epub", downloadMetadata{})

	if len(client.send) != 3 {
		t.Fatalf("expected 3 notifications, got %d", len(client.send))
	}

	first := (<-client.send).(StatusResponse)
	second := (<-client.send).(StatusResponse)
	third := (<-client.send).(StatusResponse)

	if first.NotificationType != SUCCESS || first.Title != "Processed" {
		t.Fatalf("unexpected first notification: %+v", first)
	}
	if second.NotificationType != WARNING || second.Title != "Skipped" {
		t.Fatalf("unexpected second notification: %+v", second)
	}
	if third.NotificationType != DANGER || third.Title != "Failed" {
		t.Fatalf("unexpected third notification: %+v", third)
	}
}

func TestRunPostDownloadHookIgnoresMalformedNotifications(t *testing.T) {
	originalRunner := runHookCommand
	defer func() {
		runHookCommand = originalRunner
	}()

	runHookCommand = func(_ string, _ time.Duration, _ string, _ downloadMetadata) ([]byte, bool, error) {
		output := strings.Join([]string{
			"OPENBOOKS_NOTIFY {not-json}",
			"OPENBOOKS_NOTIFY {\"level\":\"warn\",\"title\":\"Valid\"}",
		}, "\n")
		return []byte(output), false, nil
	}

	client := &Client{
		log:               log.New(io.Discard, "", 0),
		hookWorkerLimiter: make(chan struct{}, 1),
		send:              make(chan interface{}, 4),
	}

	client.runPostDownloadHook("ignored", time.Second, "/tmp/book.epub", downloadMetadata{})

	if len(client.send) != 1 {
		t.Fatalf("expected only one valid notification, got %d", len(client.send))
	}

	notify := (<-client.send).(StatusResponse)
	if notify.NotificationType != WARNING || notify.Title != "Valid" {
		t.Fatalf("unexpected notification: %+v", notify)
	}
}

func TestRunPostDownloadHookTimeoutSendsFailureNotification(t *testing.T) {
	originalRunner := runHookCommand
	defer func() {
		runHookCommand = originalRunner
	}()

	runHookCommand = func(_ string, _ time.Duration, _ string, _ downloadMetadata) ([]byte, bool, error) {
		return []byte(""), true, context.DeadlineExceeded
	}

	client := &Client{
		log:               log.New(io.Discard, "", 0),
		hookWorkerLimiter: make(chan struct{}, 1),
		send:              make(chan interface{}, 2),
	}

	client.runPostDownloadHook("/opt/hooks/process.py", 2*time.Second, "/books/book.epub", downloadMetadata{})

	if len(client.send) != 1 {
		t.Fatalf("expected timeout notification, got %d messages", len(client.send))
	}

	status, ok := (<-client.send).(StatusResponse)
	if !ok {
		t.Fatal("expected status response for timeout")
	}
	if status.NotificationType != DANGER {
		t.Fatalf("expected danger notification, got %v", status.NotificationType)
	}
	if !strings.Contains(status.Detail, "/opt/hooks/process.py /books/book.epub") {
		t.Fatalf("unexpected timeout detail: %q", status.Detail)
	}
}
