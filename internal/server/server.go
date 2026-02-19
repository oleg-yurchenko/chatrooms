package server

import (
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/oleg-yurchenko/chatrooms/internal/shared"
)

type Conn struct {
	ctx  context.Context
	uid  shared.UserId
	name string
	kp   shared.KeyPair
	conn net.Conn
	rcv  *gob.Decoder
	snd  *gob.Encoder
}

type Server struct {
	users      map[string]shared.UserId
	conns      map[shared.UserId]*Conn
	listener   net.Listener
	name       string
	anonTicker uint
	logMsgs    chan string
}

type ServerConfig struct {
	Name string
	Addr string
	Port int
}

func MakeServer(cfg ServerConfig) (server *Server, err error) {
	server = &Server{
		users:      make(map[string]shared.UserId),
		conns:      make(map[shared.UserId]*Conn),
		name:       cfg.Name,
		anonTicker: 0,
		logMsgs:    make(chan string, 256),
	}

	server.listener, err = net.Listen("tcp", net.JoinHostPort(cfg.Addr, strconv.Itoa(cfg.Port)))
	if err != nil {
		return
	}
	log.Printf("Spun up server on %v", server.listener.Addr())

	return
}

func (server *Server) Start() error {
	defer server.listener.Close()

	newConn := make(chan net.Conn)
	go func() {
		for {
			conn, err := server.listener.Accept()
			if err == nil {
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
		case msg := <-server.logMsgs:
			log.Println(msg)
		}
	}
}

// goroutine that establishes a connection to a new user and processes it
func (server *Server) Establish(ctx context.Context, conn net.Conn) {
	server.logMsgs <- fmt.Sprintf("Received connection request from %v (local: %v)", conn.RemoteAddr(), conn.LocalAddr())

	var err error = nil
	uid := shared.MakeUserId()
	defer func() {
		if err != nil {
			delete(server.conns, uid)
		}
	}()

	var kp shared.ECDHKeyPair
	err = kp.Init()
	if err != nil {
		log.Printf("%v", err)
		return
	}

	newConn := &Conn{
		ctx:  ctx,
		uid:  uid,
		name: "anon",
		kp:   &kp,
		conn: conn,
		rcv:  gob.NewDecoder(conn),
		snd:  gob.NewEncoder(conn),
	}

	// the first message to be sent should be the user's public key. We want to store this, and send ours back
	pubmsg := &shared.EstablishMessage{}
	err = newConn.rcv.Decode(pubmsg)
	if err != nil {
		return
	}

	err = newConn.kp.UnmarshalPublic(pubmsg.Pubkey)
	if err != nil {
		return
	}

	err = newConn.kp.Exchange()
	if err != nil {
		return
	}

	// check the user's desired name. If the name is already in use, assign an anon name
	uname := pubmsg.Name
	if _, exists := server.users[uname]; exists {
		// assign name to be anon
		server.anonTicker++
		uname = "anon" + strconv.Itoa(int(server.anonTicker))
	}

	newConn.name = uname

	// update the registry
	server.conns[uid] = newConn
	server.users[uname] = uid

	// send the user their assigned name + our public key
	var pkey []byte
	pkey, err = newConn.kp.MarshalPublic()
	if err != nil {
		return
	}
	ourmsg := &shared.EstablishMessage{
		Name:   uname,
		Pubkey: pkey,
	}
	newConn.snd.Encode(ourmsg)

	// at this point, we want to enter our main loop, let's send a welcome message
	welcome := &shared.Message{
		SenderId: 0,
		Nickname: server.name,
		Cmd:      shared.SendMessage,
		Data:     fmt.Sprintf("Welcome, %s!", uname),
	}
	encrypted := newConn.kp.EncryptMessage(*welcome)
	newConn.snd.Encode(encrypted)

	return
}
