package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

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
	To     string `json:"to"`
	Data   string `json:"data"`
	client *Client
	server *ChatServer
}

// Message a message
type Message struct {
	Body string
	From *Client
	To   *Client
}

func clientHandler(ws *websocket.Conn, server *ChatServer) http.HandlerFunc {
	client := Client{ID: strconv.Itoa(len(server.Clients) + 1), Conn: ws}
	fmt.Printf("Client connected. ID given: %v\n", client.ID)
	server.Clients[client.ID] = &client
	for {
		rawMsg := RawMessage{client: &client, server: server}
		err := websocket.JSON.Receive(client.Conn, &rawMsg)
		rawMsg.err = err
		handleIncomingMessage(rawMsg, err)
	}
}

func handleIncomingMessage(rawMsg RawMessage, err error) {
	if err != nil {
		panic("Failed to receive message: " + err.Error())
	}

	// Determine destination
	msg, err := parseMessage(rawMsg)
	fmt.Printf("Received message from id: %v: %s\n", rawMsg.client.ID, msg.Body)

	if err != nil {
		outMsg := RawMessage{To: rawMsg.client.ID, Data: "Error: invalid message format"}
		err := websocket.JSON.Send(rawMsg.client.Conn, &outMsg)
		if err != nil {
			panic("Failed to send message: " + err.Error())
		}
	} else {
		if msg.To != nil {
			fmt.Printf("Sending Message: %v\n", msg)
			outMsg := RawMessage{To: msg.To.ID, Data: msg.Body}
			err := websocket.JSON.Send(msg.To.Conn, &outMsg)
			if err != nil {
				panic("Failed to send message: " + err.Error())
			}
		}
	}
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
	msg := Message{Body: rawMsg.Data, From: rawMsg.client, To: to}
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
