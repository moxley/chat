package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/moxley/chat/chat"

	"golang.org/x/net/websocket"
)

// Message a message
type Message struct {
	Body  string
	From  *chat.Client
	To    *chat.Client
	ToStr string
}

func clientHandler(ws *websocket.Conn, server *chat.Server) {
	client := server.CreateAndRegisterClient(ws)
	for {
		err := receiveFrame(client, server)
		if err != nil {
			break
		}
	}
}

func receiveFrame(cli *chat.Client, server *chat.Server) error {
	frame := &chat.Frame{FromClient: cli}
	rawFrame := chat.RawFrame{}
	err := websocket.JSON.Receive(cli.Conn, &rawFrame)
	frame.Data = rawFrame.Data
	frame.Action = rawFrame.Action
	frame.To = rawFrame.To
	server.Logger.Println("Incoming message")
	return handleIncomingMessage(server, frame, err)
}

func allClientsExceptID(server *chat.Server, id string) []*chat.Client {
	var chosenClients []*chat.Client
	for _, cli := range server.AllClients() {
		if cli.ID != id {
			chosenClients = append(chosenClients, cli)
		}
	}
	return chosenClients
}

func calculateReceivers(msg *Message, server *chat.Server) []*chat.Client {
	if msg.ToStr == "all" {
		return server.AllClients()
	} else if msg.To == nil {
		return []*chat.Client{}
	}
	return []*chat.Client{msg.To}
}

func handleSetName(server *chat.Server, frame *chat.Frame) error {
	frame.FromClient.Name = frame.Data
	outMsg := chat.Frame{
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
		outMsg := chat.Frame{
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

func sendSystemMessage(server *chat.Server, to *chat.Client, msg string) error {
	msgFrame := &chat.Frame{
		ToClient: to,
		To:       to.ID,
		FromName: "auto-reply",
		Data:     msg,
	}
	return msgFrame.Send(server)
}

func sendRegularMessage(server *chat.Server, from *chat.Client, to *chat.Client, msg string) error {
	server.Logger.Printf("Sending regular message: %v\n", msg)
	msgFrame := &chat.Frame{
		From:       from.ID,
		FromClient: from,
		FromName:   from.Name,
		ToClient:   to,
		To:         to.ID,
		Data:       msg,
	}
	return msgFrame.Send(server)
}

func handleRegularMessage(server *chat.Server, frame *chat.Frame) error {
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

func handleIncomingMessage(server *chat.Server, frame *chat.Frame, err error) error {
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

	server.Logger.Printf("Received message. frame: %v\n", frame)

	if frame.Action == "set-name" {
		err = handleSetName(server, frame)
	} else {
		err = handleRegularMessage(server, frame)
	}
	return err
}

func parseMessage(server *chat.Server, frame *chat.Frame) (*Message, error) {
	var to *chat.Client
	var parseError error

	// TODO Better validation
	if frame.To == "" {
		parseError = errors.New("No destination specified")
		to = nil
	}

	msg := Message{Body: frame.Data, From: frame.FromClient, To: to, ToStr: frame.To}
	return &msg, parseError
}

func listen(server *chat.Server, port int, finished chan bool) (net.Listener, error) {
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

// NewWebsocketHTTPHandler creates an http handler for the websocket handler
func NewWebsocketHTTPHandler(server *chat.Server) http.Handler {
	chatHandler := func(ws *websocket.Conn) {
		clientHandler(ws, server)
	}
	return websocket.Handler(chatHandler)
}

func echoHandler(ws *websocket.Conn) {
	defer ws.Close()
	io.Copy(ws, ws)
}

// StartServer starts the chat server
func StartServer(server *chat.Server) (chan bool, net.Listener, error) {
	http.Handle("/websocket", NewWebsocketHTTPHandler(server))
	http.Handle("/echo", websocket.Handler(echoHandler))
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
	server := chat.NewServer(nil)
	finished, _, err := StartServer(server)
	if err != nil {
		return
	}
	<-finished
}
