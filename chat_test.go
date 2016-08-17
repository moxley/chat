package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"golang.org/x/net/websocket"

	"github.com/moxley/chat/chatserver"
)

var serverAddr string
var once sync.Once

func createServer() *chatserver.ChatServer {
	var buf bytes.Buffer
	logger := log.New(&buf, "Test:", log.Lshortfile)
	config := &chatserver.Config{Logger: logger, Port: 8001}
	return chatserver.NewServer(config)
}

func echoServer(ws *websocket.Conn) {
	defer ws.Close()
	io.Copy(ws, ws)
}

func startServer() {
	chatServer := createServer()
	handler := NewWebsocketHTTPHandler(chatServer)
	http.Handle("/websocket", handler)
	server := httptest.NewServer(nil)
	serverAddr = server.Listener.Addr().String()
	log.Print("Test WebSocket server listening on ", serverAddr)
}

func newConfig(t *testing.T, path string) *websocket.Config {
	config, _ := websocket.NewConfig(fmt.Sprintf("ws://%s%s", serverAddr, path), "http://localhost")
	return config
}

func TestEcho(t *testing.T) {
	once.Do(startServer)

	client, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatal("dialing", err)
	}
	conn, err := websocket.NewClient(newConfig(t, "/websocket"), client)
	if err != nil {
		t.Errorf("WebSocket handshake error: %v", err)
		return
	}

	// Send message
	// {"action":"set-name","data":"moxley"}
	sendFrame := &chatserver.Frame{Action: "set-name", Data: "moxley"}
	err = websocket.JSON.Send(conn, sendFrame)
	if err != nil {
		t.Errorf("Write: %v", err)
	}

	// Receive message
	var frame = chatserver.Frame{}
	err = websocket.JSON.Receive(conn, &frame)
	if err != nil {
		t.Errorf("Read: %v", err)
	}

	// {"from":"","fromName":"auto-reply","to":"28ae473a-a1f3-41b2-44c9-b47814c6f0c2","data":"Welcome moxley","action":"","FromClient":null,"ToClient":null}

	if frame.FromName != "auto-reply" {
		t.Errorf("Reply: expected %q, got %q", "auto-reply", frame.FromName)
	}
	if frame.Data != "Welcome moxley" {
		t.Errorf("Reply: expected %q, got %q", "Welcome moxley", frame.Data)
	}

	conn.Close()
}
