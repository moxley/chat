package main

import (
	"bytes"
	"log"
	"testing"

	"github.com/moxley/chat/chatserver"
)

func TestRegistration(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "Test:", log.Lshortfile)
	config := &chatserver.Config{Logger: logger}
	server := chatserver.NewServer(config)
	_, _, err := StartServer(server)
	if err != nil {
		t.Error(err)
	}
}
