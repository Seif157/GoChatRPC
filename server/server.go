package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"
)

type Message struct {
	ID      int
	Sender  string
	Content string
	Time    time.Time
}

type User struct {
	ID       string
	LastSeen int
	Joined   time.Time
	Original string
}

type ChatService struct {
	mu      sync.RWMutex
	users   map[string]*User
	history []Message
	nextID  int
	nameMap map[string]int
}

func NewChatService() *ChatService {
	return &ChatService{
		users:   make(map[string]*User),
		history: []Message{},
		nextID:  1,
		nameMap: make(map[string]int),
	}
}

type JoinArgs struct {
	RequestedName string
}

type JoinReply struct {
	Success      bool
	AssignedName string
	Message      string
}

type SendArgs struct {
	ID      string
	Message string
}

type SendReply struct {
	Success bool
}

type UpdateArgs struct {
	ID        string
	LastMsgID int
}

type UpdateReply struct {
	Messages []Message
	NewMsgID int
}

// Generate unique username
func (cs *ChatService) uniqueName(base string) string {
	if base == "" {
		base = "Guest"
	}

	if _, exists := cs.users[base]; !exists {
		return base
	}

	for i := 1; i < 100; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if _, exists := cs.users[candidate]; !exists {
			return candidate
		}
	}

	return fmt.Sprintf("%s_%d", base, time.Now().UnixNano()%9999)
}

func (cs *ChatService) Join(args JoinArgs, reply *JoinReply) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	name := cs.uniqueName(args.RequestedName)
	cs.users[name] = &User{ID: name, LastSeen: cs.nextID - 1, Joined: time.Now(), Original: args.RequestedName}

	sysMsg := Message{
		ID:      cs.nextID,
		Sender:  "System",
		Content: fmt.Sprintf("User %s joined the chat", name),
		Time:    time.Now(),
	}
	cs.nextID++
	cs.history = append(cs.history, sysMsg)

	reply.Success = true
	reply.AssignedName = name
	reply.Message = fmt.Sprintf("Welcome, %s!", name)

	fmt.Printf("[JOIN] %s connected\n", name)
	return nil
}

func (cs *ChatService) Send(args SendArgs, reply *SendReply) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	user, ok := cs.users[args.ID]
	if !ok {
		return fmt.Errorf("user not found")
	}

	msg := Message{ID: cs.nextID, Sender: args.ID, Content: args.Message, Time: time.Now()}
	cs.nextID++
	cs.history = append(cs.history, msg)
	user.LastSeen = msg.ID

	fmt.Printf("[MSG] %s: %s\n", args.ID, args.Message)
	reply.Success = true
	return nil
}

func (cs *ChatService) GetUpdates(args UpdateArgs, reply *UpdateReply) error {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if _, ok := cs.users[args.ID]; !ok {
		return fmt.Errorf("unknown user")
	}

	newMessages := []Message{}
	maxID := args.LastMsgID

	for _, m := range cs.history {
		if m.ID > args.LastMsgID && m.Sender != args.ID {
			newMessages = append(newMessages, m)
			maxID = m.ID
		}
	}

	reply.Messages = newMessages
	reply.NewMsgID = maxID
	return nil
}

func (cs *ChatService) Leave(args struct{ ID string }, reply *JoinReply) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, exists := cs.users[args.ID]; !exists {
		return nil
	}

	delete(cs.users, args.ID)

	leaveMsg := Message{ID: cs.nextID, Sender: "System", Content: fmt.Sprintf("User %s left the chat", args.ID), Time: time.Now()}
	cs.nextID++
	cs.history = append(cs.history, leaveMsg)

	reply.Success = true
	reply.Message = "Left chat successfully"
	fmt.Printf("[LEAVE] %s disconnected\n", args.ID)
	return nil
}

func main() {
	cs := NewChatService()
	rpc.Register(cs)

	listener, err := net.Listen("tcp", "127.0.0.1:1234")
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	fmt.Println("Chat server is running on port 1234...")
	fmt.Println("Waiting for clients...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Connection error:", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}
