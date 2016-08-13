package chatserver

// ClientsRegistry A thread-safe map of ID->Client pairs
import (
	"fmt"
	"sync"

	"golang.org/x/net/websocket"
)

type ClientsRegistry struct {
	sync.RWMutex
	m map[string]*Client
}

// ChatServer server context
type ChatServer struct {
	// Clients map[string]*Client
	Clients ClientsRegistry
}

func New() *ChatServer {
	server := ChatServer{}
	server.Clients.m = make(map[string]*Client)
	return &server
}

func (registry *ClientsRegistry) asArray() []*Client {
	registry.RLock()
	v := make([]*Client, len(registry.m), len(registry.m))
	idx := 0
	for _, value := range registry.m {
		v[idx] = value
		idx++
	}
	registry.RUnlock()
	return v
}

func (server *ChatServer) allClients() []*Client {
	return server.Clients.asArray()
}

func (server *ChatServer) registerClient(client *Client) {
	// Register client
	server.Clients.Lock()
	server.Clients.m[client.ID] = client
	server.Clients.Unlock()
}

func (server *ChatServer) createAndRegisterClient(ws *websocket.Conn) *Client {
	client := createClient(ws)
	server.registerClient(client)

	fmt.Printf("Client connected. ID given: %v\n", client.ID)

	return client
}

func (server *ChatServer) findClient(id string) *Client {
	server.Clients.RLock()
	fetchedClient := server.Clients.m[id]
	server.Clients.RUnlock()
	return fetchedClient
}

func (server *ChatServer) destroyClient(client *Client) {
	server.Clients.Lock()
	delete(server.Clients.m, client.ID)
	server.Clients.Unlock()
	client.Conn.Close()
}
