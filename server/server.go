package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/evan-buss/openbooks/irc"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/cors"
)

type server struct {
	// Shared app configuration
	config *Config

	// Shared data
	repository *Repository

	// Registered clients.
	clients map[uuid.UUID]*Client

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	log *log.Logger

	// Mutex to guard the lastSearch timestamp
	lastSearchMutex sync.Mutex

	// The time the last search was performed. Used to rate limit searches.
	lastSearch time.Time

	requestRotationMutex sync.Mutex
	requestCount         int
	rotateOnNextSearch   bool

	ircMutex          sync.Mutex
	ircConn           *irc.Conn
	ircReady          chan struct{}
	ircReadySet       bool
	ircConnecting     bool
	currentSearchUser uuid.UUID
	pendingDownloads  []pendingDownload
}

type pendingDownload struct {
	UserID   uuid.UUID
	Metadata downloadMetadata
}

// Config contains settings for server
type Config struct {
	Debug                     bool
	Log                       bool
	Port                      string
	UserName                  string
	Persist                   bool
	DownloadDir               string
	PostDownloadHook          string
	PostDownloadHookTimeout   time.Duration
	PostDownloadHookWorkers   int
	AssignRandomUsernameAfter int
	Basepath                  string
	Server                    string
	EnableTLS                 bool
	SearchTimeout             time.Duration
	SearchBot                 string
	DisableBrowserDownloads   bool
	UserAgent                 string
}

func New(config Config) *server {
	return &server{
		repository: NewRepository(),
		config:     &config,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[uuid.UUID]*Client),
		log:        log.New(os.Stdout, "SERVER: ", log.LstdFlags|log.Lmsgprefix),
		ircReady:   make(chan struct{}),
	}
}

// Start instantiates the web server and opens the browser
func Start(config Config) {
	createBooksDirectory(config)
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)

	corsConfig := cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   []string{"http://127.0.0.1:5173"},
		AllowedHeaders:   []string{"*"},
		AllowedMethods:   []string{"GET", "DELETE"},
	}
	router.Use(cors.New(corsConfig).Handler)

	server := New(config)
	routes := server.registerRoutes()

	ctx, cancel := context.WithCancel(context.Background())
	go server.startClientHub(ctx)
	server.registerGracefulShutdown(cancel)
	router.Mount(config.Basepath, routes)

	server.log.Printf("Base Path: %s\n", config.Basepath)
	server.log.Printf("OpenBooks is listening on port %v", config.Port)
	server.log.Printf("Download Directory: %s\n", config.DownloadDir)
	server.log.Printf("Open http://localhost:%v%s in your browser.", config.Port, config.Basepath)
	server.log.Fatal(http.ListenAndServe(":"+config.Port, router))
}

// The client hub is to be run in a goroutine and handles management of
// websocket client registrations.
func (server *server) startClientHub(ctx context.Context) {
	for {
		select {
		case client := <-server.register:
			server.clients[client.uuid] = client
		case client := <-server.unregister:
			if _, ok := server.clients[client.uuid]; ok {
				_, cancel := context.WithCancel(client.ctx)
				close(client.send)
				cancel()
				delete(server.clients, client.uuid)

				if len(server.clients) == 0 {
					server.ircMutex.Lock()
					if server.ircConn != nil {
						server.log.Println("No connected web clients; disconnecting shared IRC connection.")
						server.ircConn.Disconnect()
						server.ircConn = nil
					}
					server.currentSearchUser = uuid.Nil
					server.pendingDownloads = nil
					server.ircReady = make(chan struct{})
					server.ircReadySet = false
					server.ircMutex.Unlock()
				}
			}
		case <-ctx.Done():
			for _, client := range server.clients {
				_, cancel := context.WithCancel(client.ctx)
				close(client.send)
				cancel()
				delete(server.clients, client.uuid)
			}
			return
		}
	}
}

func (server *server) registerGracefulShutdown(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		server.log.Println("Graceful shutdown.")
		// Close the shutdown channel. Triggering all reader/writer WS handlers to close.
		cancel()
		time.Sleep(time.Second)
		os.Exit(0)
	}()
}

func createBooksDirectory(config Config) {
	err := os.MkdirAll(config.DownloadDir, os.FileMode(0755))
	if err != nil {
		panic(err)
	}
}

func (server *server) consumeRotateOnNextSearch() bool {
	server.requestRotationMutex.Lock()
	defer server.requestRotationMutex.Unlock()

	if !server.rotateOnNextSearch {
		return false
	}

	server.rotateOnNextSearch = false
	return true
}

func (server *server) markRequestForRotation() {
	n := server.config.AssignRandomUsernameAfter
	if n <= 0 {
		return
	}

	server.requestRotationMutex.Lock()
	defer server.requestRotationMutex.Unlock()

	server.requestCount++
	if server.requestCount%n == 0 {
		server.rotateOnNextSearch = true
	}
}
