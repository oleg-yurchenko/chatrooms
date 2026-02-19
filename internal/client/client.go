package client

import (
	"encoding/gob"
	"log"
	"net"
	"strconv"

	"github.com/oleg-yurchenko/chatrooms/internal/shared"
)

type Client struct {
	uid  shared.UserId
	name string
	conn net.Conn
	msgs chan shared.Message
	kp   shared.KeyPair
	rcv  *gob.Decoder
	snd  *gob.Encoder
}

type ClientConfig struct {
	Addr string
	Port int
	Name string
}

func MakeClient(cfg ClientConfig) (client *Client, err error) {
	client = &Client{
		uid:  0,
		name: cfg.Name,
		msgs: make(chan shared.Message, 32),
	}

	client.conn, err = net.Dial("tcp", net.JoinHostPort(cfg.Addr, strconv.Itoa(cfg.Port)))
	if err != nil {
		return
	}
	log.Printf("Dialed server on %v (local: %c)", client.conn.RemoteAddr(), client.conn.LocalAddr())

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

	return
}

func (client *Client) Exit() {
	client.conn.Close()
}
