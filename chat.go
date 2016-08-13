package main

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/nu7hatch/gouuid"

	"golang.org/x/net/websocket"
)

// Client a chat client
type Client struct {
	ID   string
	Conn *websocket.Conn
	Name string
}

// ClientsRegistry A thread-safe map of ID->Client pairs
type ClientsRegistry struct {
	sync.RWMutex
	m map[string]*Client
}

// ChatServer server context
type ChatServer struct {
	// Clients map[string]*Client
	Clients ClientsRegistry
}

// RawMessage unprocessed message
type RawMessage struct {
	err      error
	From     string `json:"from"`
	FromName string `json:"fromName"`
	To       string `json:"to"`
	Data     string `json:"data"`
	Action   string `json:"action"`
	client   *Client
}

// Message a message
type Message struct {
	Body  string
	From  *Client
	To    *Client
	ToStr string
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

func clientHandler(ws *websocket.Conn, server *ChatServer) {
	client := server.createAndRegisterClient(ws)
	for {
		rawMsg := RawMessage{client: client}
		err := websocket.JSON.Receive(client.Conn, &rawMsg)
		rawMsg.err = err
		err = handleIncomingMessage(server, rawMsg, err)
		if err != nil {
			break
		}
	}
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

func allClientsExceptID(server *ChatServer, id string) []*Client {
	var chosenClients []*Client
	for _, client := range server.allClients() {
		if client.ID != id {
			chosenClients = append(chosenClients, client)
		}
	}
	return chosenClients
}

func calculateReceivers(msg *Message, server *ChatServer) []*Client {
	if msg.ToStr == "all" {
		return server.allClients()
	} else if msg.To == nil {
		return []*Client{}
	}
	return []*Client{msg.To}
}

func handleIncomingMessage(server *ChatServer, rawMsg RawMessage, err error) error {
	if err != nil {
		fmt.Printf("Error on receving socket (client.ID=%v). Destroying client and connection.\n", rawMsg.client.ID)
		server.destroyClient(rawMsg.client)
		return errors.New("Client failed")
	}

	fmt.Printf("Received message. id: %v: to: %v, body: %v\n", rawMsg.client.ID, rawMsg.To, rawMsg.Data)

	if rawMsg.Action == "set-name" {
		rawMsg.client.Name = rawMsg.Data
		outMsg := RawMessage{
			To:       rawMsg.client.ID,
			FromName: "auto-reply",
			Data:     "Welcome " + rawMsg.client.Name,
		}
		err = websocket.JSON.Send(rawMsg.client.Conn, &outMsg)
		if err != nil {
			fmt.Println("Failed to send message: " + err.Error())
			return err
		}
		for _, client := range allClientsExceptID(server, rawMsg.client.ID) {
			fmt.Println("Sending new user notification to client name=" + client.Name)
			outMsg := RawMessage{
				To:       client.ID,
				FromName: "auto-reply",
				Data:     rawMsg.client.Name + " has joined",
			}
			err = websocket.JSON.Send(client.Conn, &outMsg)
			if err != nil {
				fmt.Println("Failed to send message: " + err.Error())
				return err
			}
		}
		return err
	}

	// Determine destination
	msg, err := parseMessage(server, rawMsg)
	receivers := calculateReceivers(&msg, server)

	if err != nil {
		outMsg := RawMessage{
			To:       rawMsg.client.ID,
			FromName: "auto-reply",
			Data:     "Error: invalid message format",
		}
		err := websocket.JSON.Send(rawMsg.client.Conn, &outMsg)
		if err != nil {
			fmt.Println("Failed to send message: " + err.Error())
			return err
		}
	} else {
		if len(receivers) == 0 {
			fmt.Printf("No receiver found for id=%v\n", rawMsg.To)
		} else {
			for _, recip := range receivers {
				fmt.Printf("Sending Message: %v\n", msg)
				outMsg := RawMessage{
					From:     rawMsg.client.ID,
					FromName: rawMsg.client.Name,
					To:       recip.ID,
					Data:     msg.Body,
				}
				err := websocket.JSON.Send(recip.Conn, &outMsg)
				if err != nil {
					fmt.Println("Failed to send message: " + err.Error())
					return err
				}
			}
		}
	}
	return nil
}

func parseMessage(server *ChatServer, rawMsg RawMessage) (Message, error) {
	var to *Client
	var parseError error
	if rawMsg.err == nil {
		parseError = nil
		to = server.findClient(rawMsg.To)

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
	server.Clients.m = make(map[string]*Client)
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
