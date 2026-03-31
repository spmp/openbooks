package server

import (
	"time"

	"github.com/evan-buss/openbooks/irc"
	"github.com/google/uuid"
)

func (server *server) resetIrcReady() {
	server.ircMutex.Lock()
	defer server.ircMutex.Unlock()

	server.ircReady = make(chan struct{})
	server.ircReadySet = false
}

func (server *server) waitForIrcReady(timeout time.Duration) bool {
	server.ircMutex.Lock()
	ready := server.ircReady
	server.ircMutex.Unlock()

	if ready == nil {
		return true
	}

	if timeout <= 0 {
		select {
		case <-ready:
			return true
		default:
			return false
		}
	}

	select {
	case <-ready:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (server *server) sharedIrcConn() *irc.Conn {
	server.ircMutex.Lock()
	defer server.ircMutex.Unlock()
	return server.ircConn
}

func (server *server) markIrcReady() {
	server.ircMutex.Lock()
	defer server.ircMutex.Unlock()

	if server.ircReadySet {
		return
	}

	if server.ircReady == nil {
		server.ircReady = make(chan struct{})
	}

	close(server.ircReady)
	server.ircReadySet = true
}

func (server *server) enqueueDownload(userID uuid.UUID, metadata downloadMetadata) {
	server.ircMutex.Lock()
	defer server.ircMutex.Unlock()

	server.pendingDownloads = append(server.pendingDownloads, pendingDownload{UserID: userID, Metadata: metadata})
}

func (server *server) dequeueDownload() (pendingDownload, bool) {
	server.ircMutex.Lock()
	defer server.ircMutex.Unlock()

	if len(server.pendingDownloads) == 0 {
		return pendingDownload{}, false
	}

	value := server.pendingDownloads[0]
	server.pendingDownloads = server.pendingDownloads[1:]
	return value, true
}

func (server *server) setCurrentSearchUser(userID uuid.UUID) {
	server.ircMutex.Lock()
	defer server.ircMutex.Unlock()

	server.currentSearchUser = userID
}

func (server *server) currentSearchRequester() uuid.UUID {
	server.ircMutex.Lock()
	defer server.ircMutex.Unlock()

	return server.currentSearchUser
}

func (server *server) clearCurrentSearchUser() uuid.UUID {
	server.ircMutex.Lock()
	defer server.ircMutex.Unlock()

	current := server.currentSearchUser
	server.currentSearchUser = uuid.Nil
	return current
}
