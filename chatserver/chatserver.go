package chatserver

import (
	"fmt"

	"github.com/moxley/chat/client"
	"github.com/nu7hatch/gouuid"

	"golang.org/x/net/websocket"
)

// ChatServer server context
type ChatServer struct {
	clients *ClientRegistry
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
func NewServer() *ChatServer {
	server := ChatServer{}
	server.clients = NewClientRegistry()
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

// CreateAndRegisterClient creates and registers a client
func (server *ChatServer) CreateAndRegisterClient(ws *websocket.Conn) *client.Client {
	client := createClient(ws)
	server.clients.Register(client)

	fmt.Printf("Client connected. ID given: %v\n", client.ID)

	return client
}

// DestroyClient removes a client from the registry
func (server *ChatServer) DestroyClient(c *client.Client) {
	server.clients.accessQueue <- func(clients map[string]*client.Client) {
		fmt.Printf("Removing client from registry: %v\n", c)
		delete(clients, c.ID)
	}
	c.Conn.Close()
}

// Send sends a message to a client
func (msg *Frame) Send() error {
	err := websocket.JSON.Send(msg.ToClient.Conn, &msg)
	if err != nil {
		fmt.Println("Failed to send message: " + err.Error())
		return err
	}
	return nil
}
