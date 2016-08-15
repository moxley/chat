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

// ClientRegistry is a registry of clients
type ClientRegistry struct {
	clients     map[string]*Client
	accessQueue chan func(map[string]*Client)
}

// ChatServer server context
type ChatServer struct {
	clients *ClientRegistry
}

// New constructs a ChatServer
func New() *ChatServer {
	server := ChatServer{}
	server.clients = &ClientRegistry{
		clients:     make(map[string]*Client),
		accessQueue: make(chan func(map[string]*Client)),
	}
	go server.clients.handleRegistryAccess()
	return &server
}

// AsArray returns the entire client list as an array
func (registry *ClientRegistry) AsArray() []*Client {
	resultChan := make(chan []*Client)
	registry.accessQueue <- func(clients map[string]*Client) {
		v := make([]*Client, len(clients), len(clients))
		idx := 0
		for _, value := range clients {
			v[idx] = value
			idx++
		}
		resultChan <- v
	}
	return <-resultChan
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

// Register registers a client
func (registry *ClientRegistry) Register(client *Client) {
	resultChan := make(chan bool)
	registry.accessQueue <- func(clients map[string]*Client) {
		registry.clients[client.ID] = client
		resultChan <- true
	}
	<-resultChan
}

// CreateAndRegisterClient creates and registers a client
func (server *ChatServer) CreateAndRegisterClient(ws *websocket.Conn) *Client {
	client := createClient(ws)
	server.clients.Register(client)

	fmt.Printf("Client connected. ID given: %v\n", client.ID)

	return client
}

// FindClient finds a client by ID
// func (server *ChatServer) FindClient(id string, resultChan chan *Client) *Client {
// 	fetchedClient := server.Clients.m[id]
// 	resultChan <- fetchedClient
// }

func (registry *ClientRegistry) handleRegistryAccess() {
	for f := range registry.accessQueue {
		f(registry.clients)
	}
}

// DestroyClient removes a client from the registry
func (server *ChatServer) DestroyClient(client *Client) {
	server.clients.accessQueue <- func(clients map[string]*Client) {
		fmt.Printf("Removing client from registry: %v\n", client)
		delete(clients, client.ID)
	}
	client.Conn.Close()
}
