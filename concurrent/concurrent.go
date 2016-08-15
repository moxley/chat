package main

import (
	"fmt"
	"time"
)

// MaxOutstanding is the number of requests that will be handled concurrently
const MaxOutstanding = 1

// Request holds the values of a request
type Request struct {
	value    string
	finished chan bool
}

var sem = make(chan int, MaxOutstanding)

func (r Request) String() string {
	return fmt.Sprintf("<Request %s>", r.value)
}

func process(r *Request) {
	time.Sleep(1 * time.Second)
}

func handle(index int, queue chan *Request) {
	for r := range queue {
		r := r
		fmt.Printf("[%d] BEGIN request: %v\n", index, r)
		process(r)
		fmt.Printf("[%d] END request: %v\n", index, r)
		r.finished <- true
	}
}

// Serve serves a request
func Serve(clientRequests chan *Request, quit chan bool) {
	// Create two request workers
	go handle(0, clientRequests)
	go handle(1, clientRequests)

	<-quit
}

func main() {
	fmt.Println("Begin")

	fmt.Println("Starting request handler")
	queue := make(chan *Request)
	quit := make(chan bool)
	go Serve(queue, quit)

	reqCount := 10
	requests := make([]*Request, reqCount)
	for i := 0; i < reqCount; i++ {
		req := Request{
			value:    fmt.Sprintf("req%d", i),
			finished: make(chan bool, 1),
		}
		requests[i] = &req
		fmt.Printf("Queuing request: %v\n", req)
		queue <- &req
	}

	for i := 0; i < reqCount; i++ {
		req := requests[i]
		<-req.finished
	}

	quit <- true

	fmt.Println("End")
}
