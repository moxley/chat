package main

import (
	"errors"
	"fmt"
	"net/http"

	chatserver "github.com/moxley/chat/chatserver"
	"github.com/moxley/chat/client"
	"golang.org/x/net/websocket"
)

// Message a message
type Message struct {
	Body  string
	From  *client.Client
	To    *client.Client
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

func receiveFrame(cli *client.Client, server *chatserver.ChatServer) error {
	frame := chatserver.Frame{FromClient: cli}
	err := websocket.JSON.Receive(cli.Conn, &frame)
	fmt.Println("Incoming message")
	return handleIncomingMessage(server, frame, err)
}

func allClientsExceptID(server *chatserver.ChatServer, id string) []*client.Client {
	var chosenClients []*client.Client
	for _, cli := range server.AllClients() {
		if cli.ID != id {
			chosenClients = append(chosenClients, cli)
		}
	}
	return chosenClients
}

func calculateReceivers(msg *Message, server *chatserver.ChatServer) []*client.Client {
	if msg.ToStr == "all" {
		return server.AllClients()
	} else if msg.To == nil {
		return []*client.Client{}
	}
	return []*client.Client{msg.To}
}

func handleSetName(server *chatserver.ChatServer, frame chatserver.Frame) error {
	frame.FromClient.Name = frame.Data
	outMsg := chatserver.Frame{
		To:       frame.FromClient.ID,
		FromName: "auto-reply",
		Data:     "Welcome " + frame.FromClient.Name,
	}
	err := websocket.JSON.Send(frame.FromClient.Conn, &outMsg)
	if err != nil {
		fmt.Println("Failed to send message: " + err.Error())
		return err
	}
	for _, cli := range allClientsExceptID(server, frame.FromClient.ID) {
		fmt.Println("Sending new user notification to client name=" + cli.Name)
		outMsg := chatserver.Frame{
			To:       cli.ID,
			FromName: "auto-reply",
			Data:     frame.FromClient.Name + " has joined",
		}
		err = websocket.JSON.Send(cli.Conn, &outMsg)
		if err != nil {
			fmt.Println("Failed to send message: " + err.Error())
			return err
		}
	}
	return err
}

func sendSystemMessage(to *client.Client, msg string) error {
	msgFrame := &chatserver.Frame{
		ToClient: to,
		To:       to.ID,
		FromName: "auto-reply",
		Data:     msg,
	}
	return msgFrame.Send()
}

func sendRegularMessage(from *client.Client, to *client.Client, msg string) error {
	fmt.Printf("Sending regular message: %v\n", msg)
	msgFrame := &chatserver.Frame{
		From:       from.ID,
		FromClient: from,
		FromName:   from.Name,
		ToClient:   to,
		To:         to.ID,
		Data:       msg,
	}
	return msgFrame.Send()
}

func handleRegularMessage(server *chatserver.ChatServer, frame chatserver.Frame) error {
	// Determine destination
	msg, err := parseMessage(server, frame)

	if err != nil {
		return sendSystemMessage(frame.FromClient, "Error: invalid message format")
	}

	receivers := calculateReceivers(msg, server)
	if len(receivers) == 0 {
		fmt.Printf("No receiver found for id=%v\n", frame.To)
	} else {
		for _, recip := range receivers {
			err = sendRegularMessage(frame.FromClient, recip, msg.Body)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func handleIncomingMessage(server *chatserver.ChatServer, frame chatserver.Frame, err error) error {
	if err != nil {
		fmt.Printf("Error on receving socket (client.ID=%v). Destroying client and connection.\n", frame.FromClient.ID)
		server.DestroyClient(frame.FromClient)
		return errors.New("Client failed")
	}

	fmt.Printf("Received message. id: %v: to: %v, body: %v\n", frame.FromClient.ID, frame.To, frame.Data)

	if frame.Action == "set-name" {
		err = handleSetName(server, frame)
	} else {
		err = handleRegularMessage(server, frame)
	}
	return err
}

func parseMessage(server *chatserver.ChatServer, frame chatserver.Frame) (*Message, error) {
	var to *client.Client
	var parseError error

	// TODO Better validation
	if frame.To == "" {
		parseError = errors.New("No destination specified")
		to = nil
	}

	msg := Message{Body: frame.Data, From: frame.FromClient, To: to, ToStr: frame.To}
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
