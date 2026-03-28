package server

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/evan-buss/openbooks/irc"
	"github.com/evan-buss/openbooks/util"
)

var joinIRC = core.Join
var startIRCReader = core.StartReader

const ircReadyTimeout = 5 * time.Second
const ircRequestReadyTimeout = 15 * time.Second

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
	c.resetIrcReady()
	handler := c.newIrcEventHandler(server)
	err := joinIRC(c.irc, server.config.Server, server.config.EnableTLS)
	if err != nil {
		c.log.Println(err)
		c.send <- newErrorResponse("Unable to connect to IRC server.")
		return
	}

	go startIRCReader(c.ctx, c.irc, handler)

	if !c.waitForIrcReadyWithRetry(ircReadyTimeout) {
		c.log.Printf("Timed out waiting for IRC readiness for username %s", c.irc.Username)
	}

	c.send <- ConnectionResponse{
		StatusResponse: StatusResponse{
			MessageType:      CONNECT,
			NotificationType: SUCCESS,
			Title:            "Welcome, connection established.",
			Detail:           fmt.Sprintf("IRC username %s", c.irc.Username),
		},
		Name: c.irc.Username,
	}
}

func (c *Client) newIrcEventHandler(server *server) core.EventHandler {
	handler := server.NewIrcEventHandler(c)

	if server.config.Log {
		logger, _, err := util.CreateLogFile(c.irc.Username, server.config.DownloadDir)
		if err != nil {
			server.log.Println(err)
		} else {
			handler[core.Message] = func(text string) { logger.Println(text) }
		}
	}

	return handler
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
	newConn := irc.New(randomUsername(12), server.config.UserAgent)
	err := joinIRC(newConn, server.config.Server, server.config.EnableTLS)
	if err != nil {
		return err
	}

	oldConn := c.irc
	c.irc = newConn
	c.resetIrcReady()
	if oldConn != nil {
		oldConn.Disconnect()
	}

	go startIRCReader(c.ctx, c.irc, c.newIrcEventHandler(server))

	if !c.waitForIrcReadyWithRetry(ircReadyTimeout) {
		c.log.Printf("Timed out waiting for IRC readiness after username rotation: %s", c.irc.Username)
	}

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

func (c *Client) waitForIrcReadyWithRetry(timeout time.Duration) bool {
	if c.waitForIrcReady(0) {
		return true
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.irc.GetUsers("ebooks")

		remaining := time.Until(deadline)
		waitWindow := time.Second
		if remaining < waitWindow {
			waitWindow = remaining
		}
		if waitWindow <= 0 {
			break
		}

		if c.waitForIrcReady(waitWindow) {
			return true
		}
	}

	return c.waitForIrcReady(0)
}

func (c *Client) ensureIrcReadyForRequest() bool {
	if c.waitForIrcReady(0) {
		return true
	}

	if c.waitForIrcReadyWithRetry(ircRequestReadyTimeout) {
		return true
	}

	c.send <- newErrorResponse("Still waiting to join #ebooks. Try again in a moment.")
	return false
}

func (c *Client) maybeRotateUsername(server *server, allowRotation bool) bool {
	if !c.rotateOnRequest {
		return true
	}

	if !allowRotation {
		return true
	}

	err := c.reconnectWithRandomUsername(server)
	if err != nil {
		c.log.Println(err)
		c.send <- newErrorResponse("Unable to rotate IRC username. Reusing current connection.")
		c.rotateOnRequest = false
		return true
	}

	c.rotateOnRequest = false
	return true
}

func (c *Client) markRequestForRotation(server *server) {
	n := server.config.AssignRandomUsernameAfter
	if n <= 0 {
		return
	}

	c.requestCount++
	if c.requestCount%n == 0 {
		c.rotateOnRequest = true
	}
}

// handle SearchRequests and send the query to the book server
func (c *Client) sendSearchRequest(s *SearchRequest, server *server) {
	if !c.ensureIrcReadyForRequest() {
		return
	}

	if !c.maybeRotateUsername(server, true) {
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

	core.SearchBook(c.irc, server.config.SearchBot, s.Query)
	c.markRequestForRotation(server)
	server.lastSearch = time.Now()

	c.send <- newStatusResponse(NOTIFY, "Search request sent.")
}

// handle DownloadRequests by sending the request to the book server
func (c *Client) sendDownloadRequest(d *DownloadRequest, server *server) {
	if !c.ensureIrcReadyForRequest() {
		return
	}

	if !c.maybeRotateUsername(server, false) {
		return
	}

	if d.Author != "" || d.Title != "" {
		c.queueDownloadMetadata(downloadMetadata{Author: d.Author, Title: d.Title})
	} else {
		c.queueDownloadMetadata(parseDownloadMetadata(d.Book))
	}
	core.DownloadBook(c.irc, d.Book)
	c.markRequestForRotation(server)
	c.send <- newStatusResponse(NOTIFY, "Download request received.")
}
