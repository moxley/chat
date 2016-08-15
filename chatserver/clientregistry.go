package chatserver

import "github.com/moxley/chat/client"

// ClientRegistry is a registry of clients
type ClientRegistry struct {
	clients     map[string]*client.Client
	accessQueue chan func(map[string]*client.Client)
}

// NewClientRegistry creates a new ClientRegistry
func NewClientRegistry() *ClientRegistry {
	clients := &ClientRegistry{
		clients:     make(map[string]*client.Client),
		accessQueue: make(chan func(map[string]*client.Client)),
	}
	go clients.handleRegistryAccess()
	return clients
}

// AsArray returns the entire client list as an array
func (registry *ClientRegistry) AsArray() []*client.Client {
	resultChan := make(chan []*client.Client)
	registry.accessQueue <- func(clients map[string]*client.Client) {
		v := make([]*client.Client, len(clients), len(clients))
		idx := 0
		for _, value := range clients {
			v[idx] = value
			idx++
		}
		resultChan <- v
	}
	return <-resultChan
}

// Register registers a client
func (registry *ClientRegistry) Register(c *client.Client) {
	resultChan := make(chan bool)
	registry.accessQueue <- func(clients map[string]*client.Client) {
		registry.clients[c.ID] = c
		resultChan <- true
	}
	<-resultChan
}

func (registry *ClientRegistry) handleRegistryAccess() {
	for f := range registry.accessQueue {
		f(registry.clients)
	}
}
