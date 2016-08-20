package chat

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/nu7hatch/gouuid"

	"golang.org/x/net/websocket"
)

// Server server context
type Server struct {
	clients *ClientRegistry
	quit    chan bool
	Config  *Config
	Logger  *log.Logger
}

// Config is used to configure Server
type Config struct {
	Port   int
	Logger *log.Logger
}

// RawFrame Represets the raw data from a frame
type RawFrame struct {
	FromID   string `json:"fromID"`
	FromName string `json:"fromName"`
	To       string `json:"to"`
	Data     string `json:"data"`
	Action   string `json:"action"`
}

func (f *RawFrame) String() string {
	return fmt.Sprintf("RawFrame{Action: %s, To: %s, FromID: %s, FromName: %s, Data: %s}", f.Action, f.To, f.FromID, f.FromName, f.Data)
}

// Frame is an incoming or outgoing Frame
type Frame struct {
	err        error
	From       string `json:"from"`
	FromName   string `json:"fromName"`
	To         string `json:"to"`
	Data       string `json:"data"`
	Action     string `json:"action"`
	FromClient *Client
	ToClient   *Client
}

func (f *Frame) String() string {
	return fmt.Sprintf("Frame{Action: %s, To: %s, From: %s, Data: %s}", f.Action, f.To, f.From, f.Data)
}

// NewServer constructs a Server
func NewServer(config *Config) *Server {
	server := Server{}
	server.clients = NewClientRegistry()
	server.quit = make(chan bool)
	if config == nil {
		server.Config = &Config{Port: 8080}
	} else {
		server.Config = config
	}
	if server.Config.Logger == nil {
		server.Logger = log.New(os.Stdout, "Server: ", log.Lshortfile)
	} else {
		server.Logger = server.Config.Logger
	}
	return &server
}

// Start starts the chat server
func (server *Server) Start() (chan bool, net.Listener, error) {
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

func receiveFrame(cli *Client, server *Server) error {
	frame := &Frame{FromClient: cli}
	rawFrame := RawFrame{}
	err := websocket.JSON.Receive(cli.Conn, &rawFrame)
	frame.Data = rawFrame.Data
	frame.Action = rawFrame.Action
	frame.To = rawFrame.To
	server.Logger.Println("Incoming message")
	return handleIncomingMessage(server, frame, err)
}

func allClientsExceptID(server *Server, id string) []*Client {
	var chosenClients []*Client
	for _, cli := range server.AllClients() {
		if cli.ID != id {
			chosenClients = append(chosenClients, cli)
		}
	}
	return chosenClients
}

func calculateReceivers(msg *Message, server *Server) []*Client {
	if msg.ToStr == "all" {
		return server.AllClients()
	} else if msg.To == nil {
		return []*Client{}
	}
	return []*Client{msg.To}
}

func handleSetName(server *Server, frame *Frame) error {
	frame.FromClient.Name = frame.Data
	outMsg := Frame{
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
		outMsg := Frame{
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

func sendSystemMessage(server *Server, to *Client, msg string) error {
	msgFrame := &Frame{
		ToClient: to,
		To:       to.ID,
		FromName: "auto-reply",
		Data:     msg,
	}
	return msgFrame.Send(server)
}

func sendRegularMessage(server *Server, from *Client, to *Client, msg string) error {
	server.Logger.Printf("Sending regular message: %v\n", msg)
	msgFrame := &Frame{
		From:       from.ID,
		FromClient: from,
		FromName:   from.Name,
		ToClient:   to,
		To:         to.ID,
		Data:       msg,
	}
	return msgFrame.Send(server)
}

func handleRegularMessage(server *Server, frame *Frame) error {
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

func handleIncomingMessage(server *Server, frame *Frame, err error) error {
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

func parseMessage(server *Server, frame *Frame) (*Message, error) {
	var to *Client
	var parseError error

	// TODO Better validation
	if frame.To == "" {
		parseError = errors.New("No destination specified")
		to = nil
	}

	msg := Message{Body: frame.Data, From: frame.FromClient, To: to, ToStr: frame.To}
	return &msg, parseError
}

func listen(server *Server, port int, finished chan bool) (net.Listener, error) {
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

func clientHandler(ws *websocket.Conn, server *Server) {
	client := server.CreateAndRegisterClient(ws)
	for {
		err := receiveFrame(client, server)
		if err != nil {
			break
		}
	}
}

// NewWebsocketHTTPHandler creates an http handler for the websocket handler
func NewWebsocketHTTPHandler(server *Server) http.Handler {
	chatHandler := func(ws *websocket.Conn) {
		clientHandler(ws, server)
	}
	return websocket.Handler(chatHandler)
}

func echoHandler(ws *websocket.Conn) {
	defer ws.Close()
	io.Copy(ws, ws)
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

// AllClients returns all clients
func (server *Server) AllClients() []*Client {
	return server.clients.AsArray()
}

// Quit tells the server to stop handling requests
func (server *Server) Quit() {
	server.quit <- true
}

// CreateAndRegisterClient creates and registers a client
func (server *Server) CreateAndRegisterClient(ws *websocket.Conn) *Client {
	client := createClient(ws)
	server.clients.Register(client)

	server.Logger.Printf("Client connected. ID given: %v\n", client.ID)

	return client
}

// DestroyClient removes a client from the registry
func (server *Server) DestroyClient(c *Client) {
	server.clients.accessQueue <- func(clients map[string]*Client) {
		server.Logger.Printf("Removing client from registry: %v\n", c)
		delete(clients, c.ID)
	}
	c.Conn.Close()
}

// Send sends a message to a client
func (f *Frame) Send(server *Server) error {
	rawFrame := RawFrame{
		FromID:   f.FromClient.ID,
		FromName: f.FromClient.Name,
		To:       f.ToClient.ID,
		Data:     f.Data,
	}
	err := websocket.JSON.Send(f.ToClient.Conn, &rawFrame)
	if err != nil {
		server.Logger.Println("Failed to send message: " + err.Error())
		return err
	}
	return nil
}
