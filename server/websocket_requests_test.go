package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/irc"
)

func TestStartIrcConnectionWaitsForReadinessSignal(t *testing.T) {
	originalJoin := joinIRC
	originalReader := startIRCReader
	defer func() {
		joinIRC = originalJoin
		startIRCReader = originalReader
	}()

	joinIRC = func(_ *irc.Conn, _ string, _ bool) error {
		return nil
	}
	startIRCReader = func(_ context.Context, _ *irc.Conn, _ core.EventHandler) {}

	client := &Client{
		irc:      irc.New("test-user", "test-agent"),
		send:     make(chan interface{}, 1),
		log:      log.New(io.Discard, "", 0),
		ctx:      context.Background(),
		ircReady: make(chan struct{}),
	}

	server := &server{
		repository: NewRepository(),
		config: &Config{
			UserName:  "test-user",
			Server:    "example:6667",
			EnableTLS: false,
			UserAgent: "test-agent",
		},
		log: log.New(io.Discard, "", 0),
	}

	done := make(chan struct{})
	go func() {
		client.startIrcConnection(server)
		close(done)
	}()

	select {
	case <-client.send:
		t.Fatal("connection response was sent before IRC was marked ready")
	case <-time.After(150 * time.Millisecond):
	}

	server.markIrcReady()

	select {
	case msg := <-client.send:
		if _, ok := msg.(ConnectionResponse); !ok {
			t.Fatalf("expected ConnectionResponse, got %T", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("expected connection response after readiness signal")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startIrcConnection did not complete")
	}
}

func TestSendSearchRequestWaitsForIrcReadiness(t *testing.T) {
	client := &Client{
		irc:      irc.New("test-user", "test-agent"),
		send:     make(chan interface{}, 2),
		log:      log.New(io.Discard, "", 0),
		ctx:      context.Background(),
		ircReady: make(chan struct{}),
	}

	server := &server{
		config: &Config{
			UserName:      "test-user",
			SearchBot:     "search",
			SearchTimeout: 0,
		},
		log:      log.New(io.Discard, "", 0),
		ircConn:  irc.New("test-user", "test-agent"),
		ircReady: make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		client.sendSearchRequest(&SearchRequest{Query: "gatsby"}, server)
		close(done)
	}()

	select {
	case <-client.send:
		t.Fatal("search request completed before IRC readiness")
	case <-time.After(150 * time.Millisecond):
	}

	server.markIrcReady()

	select {
	case msg := <-client.send:
		status, ok := msg.(StatusResponse)
		if !ok {
			t.Fatalf("expected StatusResponse, got %T", msg)
		}
		if status.Title != "Search request sent." {
			t.Fatalf("unexpected status title %q", status.Title)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected search status after readiness signal")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sendSearchRequest did not complete")
	}
}

func TestStartIrcConnectionDoesNotSendSuccessOnReadinessTimeout(t *testing.T) {
	originalJoin := joinIRC
	originalReader := startIRCReader
	originalReadyTimeout := ircReadyTimeout
	defer func() {
		joinIRC = originalJoin
		startIRCReader = originalReader
		ircReadyTimeout = originalReadyTimeout
	}()

	joinIRC = func(_ *irc.Conn, _ string, _ bool) error {
		return nil
	}
	startIRCReader = func(_ context.Context, _ *irc.Conn, _ core.EventHandler) {}
	ircReadyTimeout = 200 * time.Millisecond

	client := &Client{
		irc:      irc.New("test-user", "test-agent"),
		send:     make(chan interface{}, 2),
		log:      log.New(io.Discard, "", 0),
		ctx:      context.Background(),
		ircReady: make(chan struct{}),
	}

	server := &server{
		repository: NewRepository(),
		config: &Config{
			UserName:  "test-user",
			Server:    "example:6667",
			EnableTLS: false,
			UserAgent: "test-agent",
		},
		log: log.New(io.Discard, "", 0),
	}

	client.startIrcConnection(server)

	if len(client.send) == 0 {
		t.Fatal("expected timeout error response")
	}

	for len(client.send) > 0 {
		msg := <-client.send
		status, ok := msg.(StatusResponse)
		if !ok {
			t.Fatalf("expected StatusResponse, got %T", msg)
		}
		if status.MessageType == CONNECT {
			t.Fatal("did not expect successful connect response after readiness timeout")
		}
	}
}

func TestStartIrcConnectionJoinFailureLogsAndReturnsError(t *testing.T) {
	originalJoin := joinIRC
	originalReader := startIRCReader
	defer func() {
		joinIRC = originalJoin
		startIRCReader = originalReader
	}()

	joinIRC = func(_ *irc.Conn, _ string, _ bool) error {
		return errors.New("dial failed")
	}
	startIRCReader = func(_ context.Context, _ *irc.Conn, _ core.EventHandler) {}

	logBuffer := bytes.NewBuffer(nil)
	client := &Client{
		irc:      irc.New("test-user", "test-agent"),
		send:     make(chan interface{}, 1),
		log:      log.New(logBuffer, "", 0),
		ctx:      context.Background(),
		ircReady: make(chan struct{}),
	}

	server := &server{
		repository: NewRepository(),
		config: &Config{
			UserName:  "test-user",
			Server:    "example:6667",
			EnableTLS: false,
			UserAgent: "test-agent",
		},
		log: log.New(io.Discard, "", 0),
	}

	client.startIrcConnection(server)

	if len(client.send) != 1 {
		t.Fatalf("expected one error response, got %d", len(client.send))
	}

	msg := <-client.send
	status, ok := msg.(StatusResponse)
	if !ok {
		t.Fatalf("expected StatusResponse, got %T", msg)
	}
	if status.MessageType != STATUS || status.NotificationType != DANGER {
		t.Fatalf("expected danger status response, got %+v", status)
	}

	logs := logBuffer.String()
	if !strings.Contains(logs, "Error connecting to IRC server=example:6667") {
		t.Fatalf("expected connection failure details in log, got %q", logs)
	}
}
