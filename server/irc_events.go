package server

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/evan-buss/openbooks/core"
	"github.com/google/uuid"
)

func (server *server) NewIrcEventHandler() core.EventHandler {
	handler := core.EventHandler{}
	handler[core.SearchResult] = server.searchResultHandler(server.config.DownloadDir)
	handler[core.BookResult] = server.bookResultHandler(server.config.DownloadDir, server.config.DisableBrowserDownloads, server.config.PostDownloadHook, server.config.PostDownloadHookTimeout)
	handler[core.NoResults] = server.noResultsHandler
	handler[core.BadServer] = server.badServerHandler
	handler[core.SearchAccepted] = server.searchAcceptedHandler
	handler[core.MatchesFound] = server.matchesFoundHandler
	handler[core.Ping] = server.pingHandler
	handler[core.ServerList] = server.userListHandler(server.repository)
	handler[core.Version] = server.versionHandler(server.config.UserAgent)
	return handler
}

func (server *server) getClientByID(userID uuid.UUID) *Client {
	if userID == uuid.Nil {
		return nil
	}
	if client, ok := server.clients[userID]; ok {
		return client
	}
	return nil
}

// searchResultHandler downloads from DCC server, parses data, and sends data to client
func (server *server) searchResultHandler(downloadDir string) core.HandlerFunc {
	return func(text string) {
		userID := server.clearCurrentSearchUser()
		client := server.getClientByID(userID)
		if client == nil {
			server.log.Println("Dropping search result because no requesting client is available.")
			return
		}

		extractedPath, err := core.DownloadExtractDCCString(downloadDir, text, nil)
		if err != nil {
			client.log.Println(err)
			client.send <- newErrorResponse("Error when downloading search results.")
			return
		}

		bookResults, parseErrors, err := core.ParseSearchFile(extractedPath)
		if err != nil {
			client.log.Println(err)
			client.send <- newErrorResponse("Error when parsing search results.")
			return
		}

		if len(bookResults) == 0 && len(parseErrors) == 0 {
			client.send <- newSearchResponse([]core.BookDetail{}, []core.ParseError{})
			return
		}

		// Output all errors so parser can be improved over time
		if len(parseErrors) > 0 {
			client.log.Printf("%d Search Result Parsing Errors\n", len(parseErrors))
			for _, err := range parseErrors {
				client.log.Println(err)
			}
		}

		client.log.Printf("Sending %d search results.\n", len(bookResults))
		client.send <- newSearchResponse(bookResults, parseErrors)

		err = os.Remove(extractedPath)
		if err != nil {
			client.log.Printf("Error deleting search results file: %v", err)
		}
	}
}

// bookResultHandler downloads the book file and sends it over the websocket
func (server *server) bookResultHandler(downloadDir string, disableBrowserDownloads bool, postDownloadHook string, postDownloadHookTimeout time.Duration) core.HandlerFunc {
	return func(text string) {
		request, ok := server.dequeueDownload()
		if !ok {
			server.log.Println("Dropping book result because no pending download request exists.")
			return
		}
		client := server.getClientByID(request.UserID)
		if client == nil {
			server.log.Println("Dropping book result because requesting client disconnected.")
			return
		}

		extractedPath, err := core.DownloadExtractDCCString(downloadDir, text, nil)
		if err != nil {
			client.log.Println(err)
			client.send <- newErrorResponse("Error when downloading book.")
			return
		}

		metadata := request.Metadata
		if postDownloadHook != "" {
			go client.runPostDownloadHook(postDownloadHook, postDownloadHookTimeout, extractedPath, metadata)
		}

		client.log.Printf("Sending book entitled '%s'.\n", filepath.Base(extractedPath))
		client.send <- newDownloadResponse(extractedPath, disableBrowserDownloads)
	}
}

// NoResults is called when the server returns that nothing was found for the query
func (server *server) noResultsHandler(_ string) {
	userID := server.clearCurrentSearchUser()
	client := server.getClientByID(userID)
	if client == nil {
		return
	}
	client.send <- newSearchResponse([]core.BookDetail{}, []core.ParseError{})
}

// BadServer is called when the requested download fails because the server is not available
func (server *server) badServerHandler(_ string) {
	request, ok := server.dequeueDownload()
	if !ok {
		return
	}
	client := server.getClientByID(request.UserID)
	if client == nil {
		return
	}
	client.send <- newErrorResponse("Server is not available. Try another one.")
}

// SearchAccepted is called when the user's query is accepted into the search queue
func (server *server) searchAcceptedHandler(_ string) {
	client := server.getClientByID(server.currentSearchRequester())
	if client == nil {
		return
	}
	client.send <- newStatusResponse(NOTIFY, "Search accepted into the queue.")
}

// MatchesFound is called when the server finds matches for the user's query
func (server *server) matchesFoundHandler(num string) {
	client := server.getClientByID(server.currentSearchRequester())
	if client == nil {
		return
	}
	client.send <- newStatusResponse(NOTIFY, fmt.Sprintf("Found %s results for your query.", num))
}

func (server *server) pingHandler(serverUrl string) {
	server.ircMutex.Lock()
	ircConn := server.ircConn
	server.ircMutex.Unlock()
	if ircConn == nil {
		return
	}
	ircConn.Pong(serverUrl)
}

func (server *server) versionHandler(version string) core.HandlerFunc {
	return func(line string) {
		server.log.Printf("Sending CTCP version response: %s", line)
		server.ircMutex.Lock()
		ircConn := server.ircConn
		server.ircMutex.Unlock()
		if ircConn == nil {
			return
		}
		core.SendVersionInfo(ircConn, line, version)
	}
}

func (server *server) userListHandler(repo *Repository) core.HandlerFunc {
	return func(text string) {
		repo.servers = core.ParseServers(text)
		server.markIrcReady()

		server.ircMutex.Lock()
		ircConn := server.ircConn
		server.ircMutex.Unlock()

		if server.config.Debug && ircConn != nil {
			server.log.Printf("Debug: received user list and marked IRC ready (servers=%d username=%s)", len(repo.servers.ElevatedUsers), ircConn.Username)
		}
	}
}
