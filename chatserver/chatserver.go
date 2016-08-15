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
	clients map[string]*Client
}

// New constructs a ChatServer
func New() *ChatServer {
	server := ChatServer{}
	server.clients = make(map[string]*Client)
	return &server
}

func (server *ChatServer) asArray(resultChan chan []*Client) {
	v := make([]*Client, len(server.clients), len(server.clients))
	idx := 0
	for _, value := range server.clients {
		v[idx] = value
		idx++
	}
	resultChan <- v
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
	resultChan := make(chan []*Client)
	go server.asArray(resultChan)
	return <-resultChan
}

func (server *ChatServer) registerClient(client *Client, resultChan chan bool) {
	// Register client
	server.clients[client.ID] = client
	resultChan <- true
}

// CreateAndRegisterClient creates and registers a client
func (server *ChatServer) CreateAndRegisterClient(ws *websocket.Conn) *Client {
	client := createClient(ws)
	resultChan := make(chan bool)
	go server.registerClient(client, resultChan)
	<-resultChan

	fmt.Printf("Client connected. ID given: %v\n", client.ID)

	return client
}

// FindClient finds a client by ID
// func (server *ChatServer) FindClient(id string, resultChan chan *Client) *Client {
// 	fetchedClient := server.Clients.m[id]
// 	resultChan <- fetchedClient
// }

// Merge back into DestroyClient, as an anonymous function
func (server *ChatServer) destroyClient(client *Client, ch chan bool) {
	delete(server.clients, client.ID)
	client.Conn.Close()
	ch <- true
}

// DestroyClient removes a client from the registry
func (server *ChatServer) DestroyClient(client *Client) {
	ch := make(chan bool)
	go server.destroyClient(client, ch)
	<-ch
}
