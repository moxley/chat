package chat

// Message a message
type Message struct {
	Body  string
	From  *Client
	To    *Client
	ToStr string
}
