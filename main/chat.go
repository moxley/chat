package main

import "github.com/moxley/chat"

func main() {
	server := chat.NewServer(nil)
	finished, _, err := server.Start()
	if err != nil {
		return
	}
	<-finished
}
