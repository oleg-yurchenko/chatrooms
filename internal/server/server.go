package server

import (
	"context"
	"encoding/gob"
	"net"
	"strconv"

	"github.com/oleg-yurchenko/chatrooms/internal/shared"
)

type Conn struct {
	ctx  context.Context
	uid  shared.UserId
	pk   shared.KeyPair
	conn net.Conn
	rcv  *gob.Decoder
	snd  *gob.Encoder
}

type Server struct {
	users    map[string]shared.UserId
	conns    map[shared.UserId]*Conn
	listener net.Listener
}

type ServerConfig struct {
	port int
}

func MakeServer(cfg ServerConfig) (server Server, err error) {
	server.users = make(map[string]shared.UserId)
	server.conns = make(map[shared.UserId]*Conn)

	server.listener, err = net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(cfg.port)))
	if err != nil {
		return
	}

	return
}

func (server *Server) Start() error {
	defer server.listener.Close()

	newConn := make(chan net.Conn)
	go func() {
		for {
			conn, err := server.listener.Accept()
			if err != nil {
				newConn <- conn
			}
		}
	}()

	for {
		select {
		case <-context.Background().Done():
			// exit gracefully
			return nil
		case conn := <-newConn:
			// populate info
			go server.Establish(context.Background(), conn)
		}
	}
}

// goroutine that establishes a connection to a new user and processes it
func (server *Server) Establish(ctx context.Context, conn net.Conn) {
	var err error = nil
	uid := shared.MakeUserId()
	defer func() {
		if err != nil {
			delete(server.conns, uid)
		}
	}()

	newConn = &Conn{
		ctx:  ctx,
		uid:  uid,
		conn: conn,
		rcv:  gob.NewDecoder(conn),
		snd:  gob.NewEncoder(conn),
	}

	// the first message to be sent should be the user's public key. We want to store this, and send ours back
}
