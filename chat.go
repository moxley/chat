package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/nu7hatch/gouuid"

	"golang.org/x/net/websocket"
)

// Client a chat client
type Client struct {
	ID   string
	Conn *websocket.Conn
}

// ChatServer server context
type ChatServer struct {
	Clients map[string]*Client
}

// RawMessage unprocessed message
type RawMessage struct {
	err    error
	From   string `json:"from"`
	To     string `json:"to"`
	Data   string `json:"data"`
	client *Client
	server *ChatServer
}

// Message a message
type Message struct {
	Body  string
	From  *Client
	To    *Client
	ToStr string
}

func createClient(ws *websocket.Conn, server *ChatServer) *Client {
	u, err := uuid.NewV4()
	if err != nil {
		panic("Failed to create UUID")
	}
	id := u.String()
	client := Client{ID: id, Conn: ws}
	fmt.Printf("Client connected. ID given: %v\n", client.ID)
	server.Clients[client.ID] = &client
	return &client
}

func destroyClient(client *Client, server *ChatServer) {
	delete(server.Clients, client.ID)
	client.Conn.Close()
}

func clientHandler(ws *websocket.Conn, server *ChatServer) {
	client := createClient(ws, server)
	for {
		rawMsg := RawMessage{client: client, server: server}
		err := websocket.JSON.Receive(client.Conn, &rawMsg)
		rawMsg.err = err
		err = handleIncomingMessage(rawMsg, err)
		if err != nil {
			break
		}
	}
}

func clientsAsArray(clients map[string]*Client) []*Client {
	v := make([]*Client, len(clients), len(clients))
	idx := 0
	for _, value := range clients {
		v[idx] = value
		idx++
	}
	return v
}

func calculateReceivers(msg *Message, server *ChatServer) []*Client {
	if msg.ToStr == "all" {
		return clientsAsArray(server.Clients)
	} else if msg.To == nil {
		return []*Client{}
	}
	return []*Client{msg.To}
}

func handleIncomingMessage(rawMsg RawMessage, err error) error {
	if err != nil {
		fmt.Printf("Error on receving socket (client.ID=%v). Destroying client and connection.\n", rawMsg.client.ID)
		destroyClient(rawMsg.client, rawMsg.server)
		return errors.New("Client failed")
	}

	fmt.Printf("Received message. id: %v: to: %v, body: %v\n", rawMsg.client.ID, rawMsg.To, rawMsg.Data)

	// Determine destination
	msg, err := parseMessage(rawMsg)
	receivers := calculateReceivers(&msg, rawMsg.server)

	if err != nil {
		outMsg := RawMessage{To: rawMsg.client.ID, Data: "Error: invalid message format"}
		err := websocket.JSON.Send(rawMsg.client.Conn, &outMsg)
		if err != nil {
			panic("Failed to send message: " + err.Error())
		}
	} else {
		if len(receivers) == 0 {
			fmt.Printf("No receiver found for id=%v\n", rawMsg.To)
		} else {
			for _, recip := range receivers {
				fmt.Printf("Sending Message: %v\n", msg)
				outMsg := RawMessage{From: rawMsg.client.ID, To: recip.ID, Data: msg.Body}
				err := websocket.JSON.Send(recip.Conn, &outMsg)
				if err != nil {
					panic("Failed to send message: " + err.Error())
				}
			}
		}
	}
	return nil
}

func parseMessage(rawMsg RawMessage) (Message, error) {
	var to *Client
	var parseError error
	if rawMsg.err == nil {
		parseError = nil
		to = rawMsg.server.Clients[rawMsg.To]
	} else {
		// Could not determine destination
		parseError = errors.New("Could not determine message destination")
		to = nil
	}
	msg := Message{Body: rawMsg.Data, From: rawMsg.client, To: to, ToStr: rawMsg.To}
	return msg, parseError
}

func makeHandler(server *ChatServer) websocket.Handler {
	return func(ws *websocket.Conn) {
		clientHandler(ws, server)
	}
}

func listen() int {
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("ListenAndServe: " + err.Error())
		return 1
	}
	return 0
}

func createChatHandler() websocket.Handler {
	server := ChatServer{}
	server.Clients = make(map[string]*Client)
	return makeHandler(&server)
}

func main() {
	chatHandler := createChatHandler()

	http.Handle("/echo", websocket.Handler(chatHandler))
	http.Handle("/", http.FileServer(http.Dir("webroot")))
	fmt.Println("Starting server on port 8080")

	res := listen()

	if res != 0 {
		fmt.Println("Ending server on failure")
	} else {
		fmt.Println("Clean shutdown")
	}
}
