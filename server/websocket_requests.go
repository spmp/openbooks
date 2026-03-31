package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/irc"
)

var joinIRC = core.Join
var startIRCReader = core.StartReader

var ircReadyTimeout = 5 * time.Second
var ircRequestReadyTimeout = 15 * time.Second

// RequestHandler defines a generic handle() method that is called when a specific request type is made
type RequestHandler interface {
	handle(c *Client)
}

// messageRouter is used to parse the incoming request and respond appropriately
func (server *server) routeMessage(message Request, c *Client) {
	var obj interface{}

	switch message.MessageType {
	case SEARCH:
		obj = new(SearchRequest)
	case DOWNLOAD:
		obj = new(DownloadRequest)
	}

	err := json.Unmarshal(message.Payload, &obj)
	if err != nil {
		server.log.Printf("Invalid request payload. %s.\n", err.Error())
		c.send <- StatusResponse{
			MessageType:      STATUS,
			NotificationType: DANGER,
			Title:            "Unknown request payload.",
		}
	}

	switch message.MessageType {
	case CONNECT:
		c.startIrcConnection(server)
	case SEARCH:
		c.sendSearchRequest(obj.(*SearchRequest), server)
	case DOWNLOAD:
		c.sendDownloadRequest(obj.(*DownloadRequest), server)
	default:
		server.log.Println("Unknown request type received.")
	}
}

// handle ConnectionRequests and either connect to the server or do nothing
func (c *Client) startIrcConnection(server *server) {
	err := server.ensureSharedConnection(c)
	if err != nil {
		c.log.Printf("Error connecting shared IRC session: %v", err)
		c.send <- newErrorResponse(err.Error())
		return
	}

	ircConn := server.sharedIrcConn()
	if ircConn == nil {
		c.send <- newErrorResponse("Unable to connect to IRC server.")
		return
	}
	c.irc = ircConn

	c.send <- ConnectionResponse{
		StatusResponse: StatusResponse{
			MessageType:      CONNECT,
			NotificationType: SUCCESS,
			Title:            "Welcome, connection established.",
			Detail:           fmt.Sprintf("IRC username %s", ircConn.Username),
		},
		Name: ircConn.Username,
	}
}

func randomUsername(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			b[i] = alphabet[i%len(alphabet)]
			continue
		}
		b[i] = alphabet[n.Int64()]
	}

	return string(b)
}

func (c *Client) reconnectWithRandomUsername(server *server) error {
	if server.config.AssignRandomUsernameAfter <= 0 {
		return nil
	}

	newConn := irc.New(randomUsername(12), server.config.UserAgent)
	if c.debug {
		c.log.Printf("Debug: rotating IRC username to %s", newConn.Username)
	}
	err := joinIRC(newConn, server.config.Server, server.config.EnableTLS)
	if err != nil {
		return err
	}

	server.resetIrcReady()
	go startIRCReader(c.ctx, newConn, server.NewIrcEventHandler())

	if !server.waitForIrcReadyWithRetry(c, ircReadyTimeout) {
		newConn.Disconnect()
		return errors.New("unable to join #ebooks after username rotation")
	}

	server.ircMutex.Lock()
	oldConn := server.ircConn
	server.ircConn = newConn
	server.ircMutex.Unlock()

	if oldConn != nil {
		oldConn.Disconnect()
	}
	c.irc = newConn

	c.send <- ConnectionResponse{
		StatusResponse: StatusResponse{
			MessageType:      CONNECT,
			NotificationType: SUCCESS,
			Title:            "Connected with a rotated username.",
			Detail:           fmt.Sprintf("IRC username %s", c.irc.Username),
		},
		Name: c.irc.Username,
	}

	return nil
}

func (server *server) waitForIrcReadyWithRetry(client *Client, timeout time.Duration) bool {
	if server.waitForIrcReady(0) {
		return true
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ircConn := server.sharedIrcConn()
		if ircConn == nil {
			return false
		}
		if client != nil && client.debug {
			client.log.Printf("Debug: requesting channel user list for readiness check (username=%s)", ircConn.Username)
		}
		ircConn.GetUsers("ebooks")

		remaining := time.Until(deadline)
		waitWindow := time.Second
		if remaining < waitWindow {
			waitWindow = remaining
		}
		if waitWindow <= 0 {
			break
		}

		if server.waitForIrcReady(waitWindow) {
			return true
		}
	}

	return server.waitForIrcReady(0)
}

func (c *Client) ensureIrcReadyForRequest(server *server) bool {
	if server.waitForIrcReady(0) {
		return true
	}

	if c.debug {
		c.log.Printf("Debug: waiting for IRC readiness before processing request (username=%s)", c.irc.Username)
	}

	if server.waitForIrcReadyWithRetry(c, ircRequestReadyTimeout) {
		return true
	}

	c.send <- newErrorResponse("Still waiting to join #ebooks. Try again in a moment.")
	return false
}

func (c *Client) maybeRotateUsername(server *server, allowRotation bool) bool {
	if !allowRotation {
		return true
	}

	err := c.reconnectWithRandomUsername(server)
	if err != nil {
		c.log.Println(err)
		c.send <- newErrorResponse("Unable to rotate IRC username. Reusing current connection.")
		return true
	}

	return true
}

// handle SearchRequests and send the query to the book server
func (c *Client) sendSearchRequest(s *SearchRequest, server *server) {
	if !c.ensureIrcReadyForRequest(server) {
		return
	}

	if !c.maybeRotateUsername(server, server.consumeRotateOnNextSearch()) {
		return
	}

	ircConn := server.sharedIrcConn()
	if ircConn == nil {
		c.send <- newErrorResponse("Unable to connect to IRC server.")
		return
	}

	server.lastSearchMutex.Lock()
	defer server.lastSearchMutex.Unlock()

	nextAvailableSearch := server.lastSearch.Add(server.config.SearchTimeout)

	if time.Now().Before(nextAvailableSearch) {
		remainingSeconds := time.Until(nextAvailableSearch).Seconds()
		c.send <- newRateLimitResponse(remainingSeconds)

		return
	}

	server.setCurrentSearchUser(c.uuid)
	core.SearchBook(ircConn, server.config.SearchBot, s.Query)
	server.markRequestForRotation()
	server.lastSearch = time.Now()

	c.send <- newStatusResponse(NOTIFY, "Search request sent.")
}

// handle DownloadRequests by sending the request to the book server
func (c *Client) sendDownloadRequest(d *DownloadRequest, server *server) {
	if !c.ensureIrcReadyForRequest(server) {
		return
	}

	if !c.maybeRotateUsername(server, false) {
		return
	}

	ircConn := server.sharedIrcConn()
	if ircConn == nil {
		c.send <- newErrorResponse("Unable to connect to IRC server.")
		return
	}

	metadata := downloadMetadata{}
	if d.Author != "" || d.Title != "" {
		metadata = downloadMetadata{Author: d.Author, Title: d.Title}
	} else {
		metadata = parseDownloadMetadata(d.Book)
	}
	server.enqueueDownload(c.uuid, metadata)
	core.DownloadBook(ircConn, d.Book)
	server.markRequestForRotation()
	c.send <- newStatusResponse(NOTIFY, "Download request received.")
}

func (server *server) ensureSharedConnection(client *Client) error {
	deadline := time.Now().Add(ircReadyTimeout)

	for {
		server.ircMutex.Lock()
		if server.ircConn != nil && server.ircReadySet {
			server.ircMutex.Unlock()
			return nil
		}

		if server.ircConnecting {
			server.ircMutex.Unlock()
			if time.Now().After(deadline) {
				return errors.New("unable to connect to IRC server")
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		username := server.config.UserName
		if server.config.AssignRandomUsernameAfter > 0 && username == "" {
			username = randomUsername(12)
		}
		newConn := irc.New(username, server.config.UserAgent)
		server.ircConn = newConn
		server.ircConnecting = true
		server.ircReady = make(chan struct{})
		server.ircReadySet = false
		server.ircMutex.Unlock()

		if client != nil && client.debug {
			client.log.Printf("Debug: connecting to IRC server=%s tls=%t username=%s", server.config.Server, server.config.EnableTLS, newConn.Username)
		}

		err := joinIRC(newConn, server.config.Server, server.config.EnableTLS)
		if err != nil {
			if client != nil {
				client.log.Printf("Error connecting to IRC server=%s username=%s: %v", server.config.Server, newConn.Username, err)
			}
			server.log.Printf("Error connecting to IRC server=%s username=%s: %v", server.config.Server, newConn.Username, err)
			server.ircMutex.Lock()
			server.ircConn = nil
			server.ircConnecting = false
			server.ircMutex.Unlock()
			return errors.New("unable to connect to IRC server")
		}

		go startIRCReader(context.Background(), newConn, server.NewIrcEventHandler())

		ready := server.waitForIrcReadyWithRetry(client, ircReadyTimeout)

		server.ircMutex.Lock()
		server.ircConnecting = false
		server.ircMutex.Unlock()

		if !ready {
			newConn.Disconnect()
			server.ircMutex.Lock()
			if server.ircConn == newConn {
				server.ircConn = nil
			}
			server.ircMutex.Unlock()
			return errors.New("unable to join #ebooks. try reconnecting")
		}

		return nil
	}
}
