package client

// Client a chat client
import "golang.org/x/net/websocket"

// Client represents a client
type Client struct {
	ID   string
	Conn *websocket.Conn
	Name string
}
