package chatserver

import (
	"fmt"

	"github.com/nu7hatch/gouuid"

	"golang.org/x/net/websocket"
)

// Client a chat client
type Client struct {
	ID   string
	Conn *websocket.Conn
	Name string
}

// ChatServer server context
type ChatServer struct {
	clients *ClientRegistry
}

// NewServer constructs a ChatServer
func NewServer() *ChatServer {
	server := ChatServer{}
	server.clients = NewClientRegistry()
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
func (server *ChatServer) AllClients() []*Client {
	return server.clients.AsArray()
}

// CreateAndRegisterClient creates and registers a client
func (server *ChatServer) CreateAndRegisterClient(ws *websocket.Conn) *Client {
	client := createClient(ws)
	server.clients.Register(client)

	fmt.Printf("Client connected. ID given: %v\n", client.ID)

	return client
}

// DestroyClient removes a client from the registry
func (server *ChatServer) DestroyClient(client *Client) {
	server.clients.accessQueue <- func(clients map[string]*Client) {
		fmt.Printf("Removing client from registry: %v\n", client)
		delete(clients, client.ID)
	}
	client.Conn.Close()
}
