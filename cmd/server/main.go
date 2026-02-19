package main

import (
	"flag"
	"fmt"

	"github.com/oleg-yurchenko/chatrooms/internal/server"
)

func main() {
	fmt.Println("server")

	name := flag.String("name", "server", "Set the display name of the server")
	addr := flag.String("addr", "127.0.0.1", "Set the address for the server to listen on")
	port := flag.Int("port", 11337, "Set the port for the server to listen on")
	cfg := &server.ServerConfig{
		Name: *name,
		Addr: *addr,
		Port: *port,
	}
	serv, err := server.MakeServer(*cfg)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	serv.Start()
}
