package main

import (
	"bufio"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"strings"
	"time"
)

func main() {
	// Connect to RPC server
	rpcClient, err := rpc.Dial("tcp", "127.0.0.1:1234")
	if err != nil {
		log.Fatalf("Unable to connect to server: %v", err)
	}
	defer rpcClient.Close()

	console := bufio.NewReader(os.Stdin)

	// Ask for username
	fmt.Print("Enter a username (or leave blank for default): ")
	inputName, _ := console.ReadString('\n')
	inputName = strings.TrimSpace(inputName)

	// Join chat
	var joinResponse struct {
		Success      bool
		AssignedName string
		Message      string
	}

	err = rpcClient.Call("ChatRoom.Join",
		struct{ RequestedName string }{RequestedName: inputName},
		&joinResponse,
	)
	if err != nil || !joinResponse.Success {
		log.Fatalf("Join failed: %v %s", err, joinResponse.Message)
	}

	username := joinResponse.AssignedName
	fmt.Printf("\n%s\n", joinResponse.Message)
	fmt.Println("Type messages below. 'exit' to quit.")

	// Channel to stop background receiver
	stopChan := make(chan bool)
	lastID := 0

	// Goroutine to poll new messages
	go func() {
		for {
			select {
			case <-stopChan:
				return
			case <-time.After(200 * time.Millisecond):
				var updates struct {
					Messages []struct {
						ID      int
						Sender  string
						Content string
					}
					NewMsgID int
				}

				err := rpcClient.Call("ChatRoom.GetUpdates",
					struct {
						ID        string
						LastMsgID int
					}{ID: username, LastMsgID: lastID},
					&updates,
				)
				if err != nil {
					fmt.Println("\n[Connection lost]")
					stopChan <- true
					return
				}

				for _, m := range updates.Messages {
					if m.Sender == "System" {
						fmt.Printf("\n[SYSTEM] %s\n", m.Content)
					} else {
						fmt.Printf("\n%s: %s\n", m.Sender, m.Content)
					}
					fmt.Print("> ")
				}
				lastID = updates.NewMsgID
			}
		}
	}()

	// Ensure leave message on exit
	defer func() {
		stopChan <- true
		var leaveResp struct {
			Success bool
			Message string
		}
		rpcClient.Call("ChatRoom.Leave", struct{ ID string }{ID: username}, &leaveResp)
	}()

	// Main loop
	for {
		fmt.Print("> ")
		text, _ := console.ReadString('\n')
		text = strings.TrimSpace(text)

		if strings.ToLower(text) == "exit" {
			fmt.Println("Exiting chat...")
			break
		}

		if text == "" {
			continue
		}

		var sendResp struct{ Success bool }
		err = rpcClient.Call("ChatRoom.Send",
			struct {
				ID      string
				Message string
			}{ID: username, Message: text},
			&sendResp,
		)
		if err != nil {
			fmt.Printf("[Send error] %v\n", err)
			break
		}

		fmt.Printf("\n[You] %s\n", text)
	}
}
