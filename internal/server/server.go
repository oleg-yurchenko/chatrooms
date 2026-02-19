package server

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"syscall"

	"github.com/oleg-yurchenko/chatrooms/internal/shared"
)

type Conn struct {
	ctx    context.Context
	cancel context.CancelFunc
	uid    shared.UserId
	name   string
	kp     shared.KeyPair
	conn   net.Conn
	rcv    *gob.Decoder
	snd    *gob.Encoder
}

type Server struct {
	users      map[string]shared.UserId
	conns      map[shared.UserId]*Conn
	listener   net.Listener
	name       string
	anonTicker uint
	logMsgs    chan string
	cancel     context.CancelFunc
	mx         sync.Mutex
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

	ctx, cancel := context.WithCancel(context.Background())
	server.cancel = cancel

	for {
		select {
		case <-ctx.Done():
			// exit gracefully
			return nil
		case conn := <-newConn:
			// populate info
			go server.Establish(ctx, conn)
		case msg := <-server.logMsgs:
			log.Println(msg)
		}
	}
}

// goroutine that establishes a connection to a new user and processes it
func (server *Server) Establish(ctx context.Context, conn net.Conn) {
	server.logMsgs <- fmt.Sprintf("Received connection request from %v", conn.RemoteAddr())
	var err error = nil
	uid := shared.MakeUserId()

	var kp shared.ECDHKeyPair
	err = kp.Init()
	if err != nil {
		log.Printf("%v", err)
		return
	}

	localCtx, cancel := context.WithCancel(ctx)

	newConn := &Conn{
		ctx:    localCtx,
		cancel: cancel,
		uid:    uid,
		name:   "anon",
		kp:     &kp,
		conn:   conn,
		rcv:    gob.NewDecoder(conn),
		snd:    gob.NewEncoder(conn),
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
	for {
		_, exists := server.users[uname]
		if exists {
			// assign name to be anon
			server.anonTicker++
			uname = "anon" + strconv.Itoa(int(server.anonTicker))
		} else {
			break
		}
	}

	newConn.name = uname

	// update the registry
	server.mx.Lock()
	server.conns[uid] = newConn
	server.users[uname] = uid
	server.mx.Unlock()
	defer func() {
		server.mx.Lock()
		delete(server.conns, uid)
		delete(server.users, uname)
		server.mx.Unlock()
	}()

	// send the user their assigned name + our public key
	var pkey []byte
	pkey, err = newConn.kp.MarshalPublic()
	if err != nil {
		return
	}
	ourmsg := &shared.EstablishMessage{
		Name:   uname,
		Uid:    uid,
		Pubkey: pkey,
	}
	err = newConn.snd.Encode(ourmsg)
	if err != nil {
		return
	}

	// at this point, we want to enter our main loop, let's send a welcome message
	welcome := &shared.Message{
		SenderId: 0,
		Nickname: server.name,
		Cmd:      shared.SendMessage,
		Data:     fmt.Sprintf("Welcome, %s!", uname),
	}

	server.Broadcast(*welcome)

	server.HandleMessages(newConn)

	// when we return here, that means our connection was closed
	// call cancel in case we didn't exit safely
	newConn.cancel()

	return
}

func (server *Server) Broadcast(msg shared.Message) {
	var wg sync.WaitGroup
	for _, uid := range server.users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.sendToUser(msg, uid)
		}()
	}
	wg.Wait()
}

func (server *Server) sendToUser(msg shared.Message, uid shared.UserId) {
	conn, ok := server.conns[uid]
	if !ok {
		return
	}

	err := conn.snd.Encode(conn.kp.EncryptMessage(msg))
	if err == syscall.EPIPE {
		// remove ourselves from the server
		server.mx.Lock()
		delete(server.users, conn.name)
		delete(server.conns, uid)
		server.mx.Unlock()
	} else if err != nil {
		log.Printf("Failed to encode: %v", err)
	}

	log.Printf("Sent %v to %s", msg, conn.name)
}

func (server *Server) HandleMessages(conn *Conn) {
	msgChan := make(chan shared.Message, 16)
	go func() {
		for {
			select {
			case <-conn.ctx.Done():
				return
			default:
				var encrypted shared.EncryptedMessage
				err := conn.rcv.Decode(&encrypted)
				if err == io.EOF {
					conn.cancel()
					return
				} else if err != nil {
					log.Printf("Failed to decode: %v", err)
					continue
				}
				msgChan <- conn.kp.DecryptMessage(encrypted)
			}
		}
	}()

	for {
		select {
		case <-conn.ctx.Done():
			return
		case msg := <-msgChan:
			log.Printf("received %v", msg)

			go server.Broadcast(msg)
		}
	}
}
