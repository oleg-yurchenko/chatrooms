package client

import (
	"context"
	"encoding/gob"
	"log"
	"net"
	"strconv"

	"github.com/oleg-yurchenko/chatrooms/internal/shared"
)

type Client struct {
	ctx    context.Context
	cancel context.CancelFunc
	name   string
	uid    shared.UserId
	conn   net.Conn
	msgs   chan shared.Message
	kp     shared.KeyPair
	rcv    *gob.Decoder
	snd    *gob.Encoder
}

type ClientConfig struct {
	Addr string
	Port int
	Name string
}

func MakeClient(cfg ClientConfig) (client *Client, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	client = &Client{
		ctx:    ctx,
		cancel: cancel,
		name:   cfg.Name,
		msgs:   make(chan shared.Message, 32),
	}

	client.conn, err = net.Dial("tcp", net.JoinHostPort(cfg.Addr, strconv.Itoa(cfg.Port)))
	if err != nil {
		return
	}
	log.Printf("Dialed server on %v", client.conn.RemoteAddr())

	client.rcv = gob.NewDecoder(client.conn)
	client.snd = gob.NewEncoder(client.conn)

	return
}

func (client *Client) Establish() (err error) {
	err = nil
	defer func() {
		if err != nil {
			client.Exit()
		}
	}()

	var kp shared.ECDHKeyPair
	err = kp.Init()
	if err != nil {
		return err
	}

	client.kp = &kp

	var pkey []byte
	pkey, err = kp.MarshalPublic()
	if err != nil {
		return
	}
	ourmsg := &shared.EstablishMessage{
		Name:   client.name,
		Uid:    shared.UserId(0),
		Pubkey: pkey,
	}
	client.snd.Encode(ourmsg)

	// receive the server's public key and our name
	pubmsg := &shared.EstablishMessage{}
	err = client.rcv.Decode(pubmsg)
	if err != nil {
		return
	}

	err = kp.UnmarshalPublic(pubmsg.Pubkey)
	if err != nil {
		return
	}

	err = kp.Exchange()
	if err != nil {
		return
	}

	// set our name
	client.name = pubmsg.Name
	client.uid = pubmsg.Uid

	// start reading messages
	go client.receiveMessageLoop()

	return
}

// main loop that processes messages from the server and reads messages from the user
func (client *Client) FetchMessages() []shared.Message {
	out := make([]shared.Message, 0)

	for {
		select {
		case msg := <-client.msgs:
			out = append(out, msg)
		case <-client.ctx.Done():
			return nil
		default:
			return out
		}
	}
}

func (client *Client) receiveMessageLoop() {

	newMsg := make(chan shared.EncryptedMessage)
	go func() {
		for {
			var encrypted shared.EncryptedMessage
			err := client.rcv.Decode(&encrypted)
			if err == nil {
				newMsg <- encrypted
			}
		}
	}()

	for {
		select {
		case <-client.ctx.Done():
			return
		case encrypted := <-newMsg:
			client.msgs <- client.kp.DecryptMessage(encrypted)
		}
	}
}

func (client *Client) SendMessage(raw string) {
	if len(raw) == 0 {
		return
	}
	if raw[0] == '/' {
		// specially handle commands, do nothing for now

	} else {
		msg := &shared.Message{
			SenderId: client.uid,
			Nickname: client.name,
			Cmd:      shared.SendMessage,
			Data:     raw,
		}

		// encrypt and send the message
		encrypted := client.kp.EncryptMessage(*msg)
		client.snd.Encode(encrypted)
	}
}

func (client *Client) Exit() {
	client.cancel()
	client.conn.Close()
}
