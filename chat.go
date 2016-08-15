package main

import (
	"errors"
	"fmt"
	"net/http"

	chatserver "github.com/moxley/chat/chatserver"
	"golang.org/x/net/websocket"
)

// Frame is an incoming or outgoing Frame
type Frame struct {
	err      error
	From     string `json:"from"`
	FromName string `json:"fromName"`
	To       string `json:"to"`
	Data     string `json:"data"`
	Action   string `json:"action"`
	client   *chatserver.Client
}

// Message a message
type Message struct {
	Body  string
	From  *chatserver.Client
	To    *chatserver.Client
	ToStr string
}

func clientHandler(ws *websocket.Conn, server *chatserver.ChatServer) {
	client := server.CreateAndRegisterClient(ws)
	for {
		err := receiveFrame(client, server)
		if err != nil {
			break
		}
	}
}

func receiveFrame(client *chatserver.Client, server *chatserver.ChatServer) error {
	frame := Frame{client: client}
	err := websocket.JSON.Receive(client.Conn, &frame)
	fmt.Println("Incoming message")
	return handleIncomingMessage(server, frame, err)
}

func allClientsExceptID(server *chatserver.ChatServer, id string) []*chatserver.Client {
	var chosenClients []*chatserver.Client
	for _, client := range server.AllClients() {
		if client.ID != id {
			chosenClients = append(chosenClients, client)
		}
	}
	return chosenClients
}

func calculateReceivers(msg *Message, server *chatserver.ChatServer) []*chatserver.Client {
	if msg.ToStr == "all" {
		return server.AllClients()
	} else if msg.To == nil {
		return []*chatserver.Client{}
	}
	return []*chatserver.Client{msg.To}
}

func handleIncomingMessage(server *chatserver.ChatServer, frame Frame, err error) error {
	if err != nil {
		fmt.Printf("Error on receving socket (client.ID=%v). Destroying client and connection.\n", frame.client.ID)
		server.DestroyClient(frame.client)
		return errors.New("Client failed")
	}

	fmt.Printf("Received message. id: %v: to: %v, body: %v\n", frame.client.ID, frame.To, frame.Data)

	if frame.Action == "set-name" {
		frame.client.Name = frame.Data
		outMsg := Frame{
			To:       frame.client.ID,
			FromName: "auto-reply",
			Data:     "Welcome " + frame.client.Name,
		}
		err = websocket.JSON.Send(frame.client.Conn, &outMsg)
		if err != nil {
			fmt.Println("Failed to send message: " + err.Error())
			return err
		}
		for _, client := range allClientsExceptID(server, frame.client.ID) {
			fmt.Println("Sending new user notification to client name=" + client.Name)
			outMsg := Frame{
				To:       client.ID,
				FromName: "auto-reply",
				Data:     frame.client.Name + " has joined",
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
	msg, err := parseMessage(server, frame)

	if err != nil {
		outMsg := Frame{
			To:       frame.client.ID,
			FromName: "auto-reply",
			Data:     "Error: invalid message format",
		}
		err := websocket.JSON.Send(frame.client.Conn, &outMsg)
		if err != nil {
			fmt.Println("Failed to send message: " + err.Error())
			return err
		}
	} else {
		receivers := calculateReceivers(msg, server)
		if len(receivers) == 0 {
			fmt.Printf("No receiver found for id=%v\n", frame.To)
		} else {
			for _, recip := range receivers {
				fmt.Printf("Sending Message: %v\n", msg)
				outMsg := Frame{
					From:     frame.client.ID,
					FromName: frame.client.Name,
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

func parseMessage(server *chatserver.ChatServer, frame Frame) (*Message, error) {
	var to *chatserver.Client
	var parseError error

	// TODO Better validation
	if frame.To == "" {
		parseError = errors.New("No destination specified")
		to = nil
	}

	msg := Message{Body: frame.Data, From: frame.client, To: to, ToStr: frame.To}
	return &msg, parseError
}

func makeHandler(server *chatserver.ChatServer) websocket.Handler {
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

func createChatHandler(server *chatserver.ChatServer) websocket.Handler {
	return makeHandler(server)
}

func main() {
	server := chatserver.NewServer()
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
