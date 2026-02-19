package server

import (
	"context"
	"net"
	"strconv"

	"github.com/oleg-yurchenko/chatrooms/internal/shared"
)

type Conn struct {
	ctx  context.Context
	conn net.Conn
}

type Server struct {
	users    map[string]shared.UserId
	conns    map[shared.UserId]Conn
	listener net.Listener
}

type ServerConfig struct {
	port int
}

func MakeServer(cfg ServerConfig) (server Server, err error) {
	server.users = make(map[string]shared.UserId)
	server.conns = make(map[shared.UserId]Conn)

	server.listener, err = net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(cfg.port)))
	if err != nil {
		return
	}

	return
}

func (server *Server) Start() error {
	defer server.listener.Close()
	for {
		select {
		case <-context.Background().Done():
			// exit gracefully
			return nil

		}
	}

	return nil
}
