package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/moxley/chat/chatserver"
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
	server.Logger.Println("Incoming message")
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
		server.Logger.Println("Failed to send message: " + err.Error())
		return err
	}
	for _, cli := range allClientsExceptID(server, frame.FromClient.ID) {
		server.Logger.Println("Sending new user notification to client name=" + cli.Name)
		outMsg := chatserver.Frame{
			To:       cli.ID,
			FromName: "auto-reply",
			Data:     frame.FromClient.Name + " has joined",
		}
		err = websocket.JSON.Send(cli.Conn, &outMsg)
		if err != nil {
			server.Logger.Println("Failed to send message: " + err.Error())
			return err
		}
	}
	return err
}

func sendSystemMessage(server *chatserver.ChatServer, to *client.Client, msg string) error {
	msgFrame := &chatserver.Frame{
		ToClient: to,
		To:       to.ID,
		FromName: "auto-reply",
		Data:     msg,
	}
	return msgFrame.Send(server)
}

func sendRegularMessage(server *chatserver.ChatServer, from *client.Client, to *client.Client, msg string) error {
	server.Logger.Printf("Sending regular message: %v\n", msg)
	msgFrame := &chatserver.Frame{
		From:       from.ID,
		FromClient: from,
		FromName:   from.Name,
		ToClient:   to,
		To:         to.ID,
		Data:       msg,
	}
	return msgFrame.Send(server)
}

func handleRegularMessage(server *chatserver.ChatServer, frame chatserver.Frame) error {
	// Determine destination
	msg, err := parseMessage(server, frame)

	if err != nil {
		return sendSystemMessage(server, frame.FromClient, "Error: invalid message format")
	}

	receivers := calculateReceivers(msg, server)
	if len(receivers) == 0 {
		server.Logger.Printf("No receiver found for id=%v\n", frame.To)
	} else {
		for _, recip := range receivers {
			err = sendRegularMessage(server, frame.FromClient, recip, msg.Body)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func handleIncomingMessage(server *chatserver.ChatServer, frame chatserver.Frame, err error) error {
	if err != nil {
		if err == io.EOF {
			server.Logger.Printf("Client closed connection (client.ID=%v): %v\n", frame.FromClient.ID, err)
		} else {
			server.Logger.Printf("Error on receving socket (client.ID=%v): %v\n", frame.FromClient.ID, err)
		}
		server.Logger.Printf("Destroying client and connection.\n")
		server.DestroyClient(frame.FromClient)
		return errors.New("Client failed")
	}

	server.Logger.Printf("Received message. id: %v: to: %v, body: %v\n", frame.FromClient.ID, frame.To, frame.Data)

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

func listen(server *chatserver.ChatServer, port int, finished chan bool) (net.Listener, error) {
	portStr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", portStr)
	if err != nil {
		server.Logger.Printf("Failed to listen on port %v\n", port)
	} else {
		go func() {
			err = http.Serve(listener, nil)
			if err != nil {
				server.Logger.Println("ListenAndServe: " + err.Error())
			}
			finished <- true
		}()
	}
	return listener, err
}

// StartServer starts the chat server
func StartServer(server *chatserver.ChatServer) (chan bool, net.Listener, error) {
	chatHandler := func(ws *websocket.Conn) {
		clientHandler(ws, server)
	}

	http.Handle("/echo", websocket.Handler(chatHandler))
	http.Handle("/", http.FileServer(http.Dir("webroot")))
	server.Logger.Println("Starting server on port 8080...")

	finished := make(chan bool)
	listener, err := listen(server, server.Config.Port, finished)
	if err != nil {
		server.Logger.Printf("Failed to start server: %v\n", err)
		return nil, listener, err
	}
	server.Logger.Println("Server started")
	return finished, listener, nil
}

func main() {
	server := chatserver.NewServer(nil)
	finished, _, err := StartServer(server)
	if err != nil {
		return
	}
	<-finished
}
