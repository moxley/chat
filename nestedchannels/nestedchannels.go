package main

import "fmt"

// Request represents a request
type Request struct {
	args       []int
	f          func([]int) int
	resultChan chan int
}

func sum(a []int) (s int) {
	for _, v := range a {
		s += v
	}
	return
}

func handle(queue chan *Request) {
	for req := range queue {
		req.resultChan <- req.f(req.args)
	}
}

func channelsOfChannels() {
	clientRequests := make(chan *Request)
	go handle(clientRequests)
	channelsOfChannelsClient(clientRequests)
}

func channelsOfChannelsClient(clientRequests chan *Request) {

	request := &Request{[]int{3, 4, 5}, sum, make(chan int)}

	// Send request
	clientRequests <- request

	// Wait for response.
	fmt.Printf("answer: %d\n", <-request.resultChan)
}

func main() {
	fmt.Println("BEGIN")
	channelsOfChannels()
	fmt.Println("END")
}
