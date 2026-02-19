package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/oleg-yurchenko/chatrooms/internal/client"
)

func main() {
	fmt.Println("Client")

	name := flag.String("name", "anon", "The name to use in the chatroom")
	addr := flag.String("addr", "127.0.0.1", "IP address of the desired destination")
	port := flag.Int("port", 11337, "The port that the chatroom is listening on")

	flag.Parse()

	cfg := &client.ClientConfig{
		Addr: *addr,
		Port: *port,
		Name: *name,
	}
	user, err := client.MakeClient(*cfg)
	if err != nil {
		log.Printf("Failed to initialize connection to server: %v", err)
		return
	}

	err = user.Establish()
	if err != nil {
		log.Printf("Failed to establish connection to server: %v", err)
	}

	user.Exit()
}
