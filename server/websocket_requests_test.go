package server

import (
	"context"
	"io"
	"log"
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

	client.markIrcReady()

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
