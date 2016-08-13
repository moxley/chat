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
	Name string
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

func createChatHandler(server *ChatServer) websocket.Handler {
	return makeHandler(server)
}

func main() {
	server := chatserver.New()
	chatHandler := createChatHandler(server)

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
