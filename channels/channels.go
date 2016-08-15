package main

import (
	"fmt"
	"time"
)

func deadlock() {
	ch := make(chan bool)
	ch <- true
	<-ch
}

func waitForMessage() {
	ch := make(chan bool)
	go func() {
		fmt.Println("Sending message")
		ch <- true
	}()
	fmt.Println("Created goroutine")
	time.Sleep(1 * time.Second)
	fmt.Println("Waiting for message")
	<-ch
}

func main() {
	fmt.Println("BEGIN")
	waitForMessage()
	fmt.Println("END")
}
