## What

A chat system using Golang and websockets.

## Features

* Simple, name-only user registration.
* Post to single user
* Post to everybody

## How to Use

Go must be installed, and your environment must be set up for Go development,
including a GOPATH directory.

1. Install the chatserver command
  ```
  go install github.com/moxley/chat/cmd/chatserver
  ```
2. Run the server:
  ```
  $GOPATH/bin/chatserver
  ```
2. Open a browser to `http://localhost:8080/`
3. Provide a name
4. Start chatting

## Modifying the Code

After making modifications, run the tests like this:

```
go test
```
