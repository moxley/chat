package chat

import (
	"fmt"
	"log"
	"os"

	"github.com/nu7hatch/gouuid"

	"golang.org/x/net/websocket"
)

// Server server context
type Server struct {
	clients *ClientRegistry
	quit    chan bool
	Config  *Config
	Logger  *log.Logger
}

// Config is used to configure Server
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

func (f *RawFrame) String() string {
	return fmt.Sprintf("RawFrame{Action: %s, To: %s, FromID: %s, FromName: %s, Data: %s}", f.Action, f.To, f.FromID, f.FromName, f.Data)
}

// Frame is an incoming or outgoing Frame
type Frame struct {
	err        error
	From       string `json:"from"`
	FromName   string `json:"fromName"`
	To         string `json:"to"`
	Data       string `json:"data"`
	Action     string `json:"action"`
	FromClient *Client
	ToClient   *Client
}

func (f *Frame) String() string {
	return fmt.Sprintf("Frame{Action: %s, To: %s, From: %s, Data: %s}", f.Action, f.To, f.From, f.Data)
}

// NewServer constructs a Server
func NewServer(config *Config) *Server {
	server := Server{}
	server.clients = NewClientRegistry()
	server.quit = make(chan bool)
	if config == nil {
		server.Config = &Config{Port: 8080}
	} else {
		server.Config = config
	}
	if server.Config.Logger == nil {
		server.Logger = log.New(os.Stdout, "Server: ", log.Lshortfile)
	} else {
		server.Logger = server.Config.Logger
	}
	return &server
}

func createClient(ws *websocket.Conn) *Client {
	u, err := uuid.NewV4()
	if err != nil {
		panic("Failed to create UUID")
	}

	id := u.String()

	client := Client{ID: id, Conn: ws}

	return &client
}

// AllClients returns all clients
func (server *Server) AllClients() []*Client {
	return server.clients.AsArray()
}

// Quit tells the server to stop handling requests
func (server *Server) Quit() {
	server.quit <- true
}

// CreateAndRegisterClient creates and registers a client
func (server *Server) CreateAndRegisterClient(ws *websocket.Conn) *Client {
	client := createClient(ws)
	server.clients.Register(client)

	server.Logger.Printf("Client connected. ID given: %v\n", client.ID)

	return client
}

// DestroyClient removes a client from the registry
func (server *Server) DestroyClient(c *Client) {
	server.clients.accessQueue <- func(clients map[string]*Client) {
		server.Logger.Printf("Removing client from registry: %v\n", c)
		delete(clients, c.ID)
	}
	c.Conn.Close()
}

// Send sends a message to a client
func (f *Frame) Send(server *Server) error {
	rawFrame := RawFrame{
		FromID:   f.FromClient.ID,
		FromName: f.FromClient.Name,
		To:       f.ToClient.ID,
		Data:     f.Data,
	}
	err := websocket.JSON.Send(f.ToClient.Conn, &rawFrame)
	if err != nil {
		server.Logger.Println("Failed to send message: " + err.Error())
		return err
	}
	return nil
}
