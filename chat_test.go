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
	"time"

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

func startServerOnce() {
	once.Do(startServer)
}

func startClient(t *testing.T) *websocket.Conn {
	tcpConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatal("dialing", err)
	}
	wsConn, err := websocket.NewClient(newConfig(t, "/websocket"), tcpConn)
	if err != nil {
		t.Errorf("WebSocket handshake error: %v", err)
	}
	return wsConn
}

func register(t *testing.T, clientConn *websocket.Conn, userName string) string {
	// Send registration message
	// {"action":"set-name","data":"moxley"}
	sendFrame := &chatserver.Frame{Action: "set-name", Data: userName}
	err := websocket.JSON.Send(clientConn, sendFrame)
	if err != nil {
		t.Errorf("Write: %v", err)
	}

	// Receive message
	var frame = chatserver.Frame{}
	err = websocket.JSON.Receive(clientConn, &frame)
	if err != nil {
		t.Errorf("Read: %v", err)
	}

	// {"from":"","fromName":"auto-reply","to":"28ae473a-a1f3-41b2-44c9-b47814c6f0c2","data":"Welcome moxley","action":"","FromClient":null,"ToClient":null}

	if frame.FromName != "auto-reply" {
		t.Errorf("Reply: expected %q, got %q", "auto-reply", frame.FromName)
	}
	expectedMsg := fmt.Sprintf("Welcome %s", userName)
	if frame.Data != expectedMsg {
		t.Errorf("Reply: expected %q, got %q", expectedMsg, frame.Data)
	}

	return frame.To
}

// func TestRegistration(t *testing.T) {
// 	startServerOnce()
// 	clientConn := startClient(t)
// 	register(t, clientConn, "moxley")
// 	clientConn.Close()
// }

func TestRegularMessage(t *testing.T) {
	var err error
	startServerOnce()

	moxleyConn := startClient(t)
	moxleyID := register(t, moxleyConn, "moxley")
	// register(t, moxleyConn, "moxley")

	opheliaConn := startClient(t)
	register(t, opheliaConn, "ophelia")

	sendFrame := &chatserver.Frame{To: "all", Data: "Hello"}
	err = websocket.JSON.Send(moxleyConn, sendFrame)
	if err != nil {
		t.Errorf("Write: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	receiveFrame := &chatserver.RawFrame{}
	err = websocket.JSON.Receive(opheliaConn, receiveFrame)
	if err != nil {
		t.Errorf("Write: %v", err)
	}

	if receiveFrame.FromID != moxleyID {
		t.Errorf("receiveFrame.From: expected %q, got %v", moxleyID, receiveFrame.FromID)
	}
	if receiveFrame.FromName != "moxley" {
		t.Errorf("receiveFrame.From: expected %q, got %v", "moxley", receiveFrame.FromName)
	}

	moxleyConn.Close()
	opheliaConn.Close()
}
