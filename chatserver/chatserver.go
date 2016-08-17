package chatserver

import (
	"log"
	"os"

	"github.com/moxley/chat/client"
	"github.com/nu7hatch/gouuid"

	"golang.org/x/net/websocket"
)

// ChatServer server context
type ChatServer struct {
	clients *ClientRegistry
	quit    chan bool
	Config  *Config
	Logger  *log.Logger
}

// Config is used to configure ChatServer
type Config struct {
	Port   int
	Logger *log.Logger
}

// RawFrame Represets the raw data from a frame
type RawFrame struct {
	FromID   string `json:"fromID"`
	FromName string `json:"fromName"`
	To       string `json:"to"`
	Data     string `json:"data"`
	Action   string `json:"action"`
}

// Frame is an incoming or outgoing Frame
type Frame struct {
	err        error
	From       string `json:"from"`
	FromName   string `json:"fromName"`
	To         string `json:"to"`
	Data       string `json:"data"`
	Action     string `json:"action"`
	FromClient *client.Client
	ToClient   *client.Client
}

// NewServer constructs a ChatServer
func NewServer(config *Config) *ChatServer {
	server := ChatServer{}
	server.clients = NewClientRegistry()
	server.quit = make(chan bool)
	if config == nil {
		server.Config = &Config{Port: 8080}
	} else {
		server.Config = config
	}
	if server.Config.Logger == nil {
		server.Logger = log.New(os.Stdout, "ChatServer: ", log.Lshortfile)
	} else {
		server.Logger = server.Config.Logger
	}
	return &server
}

func createClient(ws *websocket.Conn) *client.Client {
	u, err := uuid.NewV4()
	if err != nil {
		panic("Failed to create UUID")
	}

	id := u.String()

	client := client.Client{ID: id, Conn: ws}

	return &client
}

// AllClients returns all clients
func (server *ChatServer) AllClients() []*client.Client {
	return server.clients.AsArray()
}

// Quit tells the server to stop handling requests
func (server *ChatServer) Quit() {
	server.quit <- true
}

// CreateAndRegisterClient creates and registers a client
func (server *ChatServer) CreateAndRegisterClient(ws *websocket.Conn) *client.Client {
	client := createClient(ws)
	server.clients.Register(client)

	server.Logger.Printf("Client connected. ID given: %v\n", client.ID)

	return client
}

// DestroyClient removes a client from the registry
func (server *ChatServer) DestroyClient(c *client.Client) {
	server.clients.accessQueue <- func(clients map[string]*client.Client) {
		server.Logger.Printf("Removing client from registry: %v\n", c)
		delete(clients, c.ID)
	}
	c.Conn.Close()
}

// Send sends a message to a client
func (msg *Frame) Send(server *ChatServer) error {
	err := websocket.JSON.Send(msg.ToClient.Conn, &msg)
	if err != nil {
		server.Logger.Println("Failed to send message: " + err.Error())
		return err
	}
	return nil
}
