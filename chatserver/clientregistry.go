package chatserver

// ClientRegistry is a registry of clients
type ClientRegistry struct {
	clients     map[string]*Client
	accessQueue chan func(map[string]*Client)
}

// NewClientRegistry creates a new ClientRegistry
func NewClientRegistry() *ClientRegistry {
	clients := &ClientRegistry{
		clients:     make(map[string]*Client),
		accessQueue: make(chan func(map[string]*Client)),
	}
	go clients.handleRegistryAccess()
	return clients
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

// Register registers a client
func (registry *ClientRegistry) Register(client *Client) {
	resultChan := make(chan bool)
	registry.accessQueue <- func(clients map[string]*Client) {
		registry.clients[client.ID] = client
		resultChan <- true
	}
	<-resultChan
}

func (registry *ClientRegistry) handleRegistryAccess() {
	for f := range registry.accessQueue {
		f(registry.clients)
	}
}
