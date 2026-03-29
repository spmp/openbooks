package server

import "testing"

func TestRequestRotationStateIsGlobalAndSearchTriggered(t *testing.T) {
	server := &server{
		config: &Config{AssignRandomUsernameAfter: 3},
	}

	if server.consumeRotateOnNextSearch() {
		t.Fatal("did not expect rotation before requests")
	}

	server.markRequestForRotation()
	server.markRequestForRotation()
	if server.consumeRotateOnNextSearch() {
		t.Fatal("did not expect rotation before threshold")
	}

	server.markRequestForRotation()
	if !server.consumeRotateOnNextSearch() {
		t.Fatal("expected rotation at threshold")
	}

	if server.consumeRotateOnNextSearch() {
		t.Fatal("expected rotation flag to reset after consume")
	}
}
