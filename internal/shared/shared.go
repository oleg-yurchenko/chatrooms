package shared

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/x509"
	"encoding/gob"
	mrand "math/rand/v2"
)

// UserId is a unique ID for a user upon joining a chatroom
type UserId uint64

func MakeUserId() UserId {
	return UserId(mrand.Uint64())
}

type ChatCommand uint8

const (
	SendMessage ChatCommand = iota
	ListOnline
	Whisper
	CopyLast
)

type Message struct {
	SenderId UserId
	Nickname string
	Cmd      ChatCommand
	Data     string
}

type EncryptedMessage struct {
	Iv   []byte
	Emsg []byte // encrypted `Message` struct
}

// this message is exclusively used when establishing a connection to send over the public key
type EstablishMessage struct {
	Name   string // acts as the user's desired name if sent from client. Acts as the assigned name when sent from server
	Pubkey []byte
}

type KeyPair interface {
	Init() error
	// reads own public key stored in the type
	MarshalPublic() ([]byte, error)
	// writes to other public key stored in the type
	UnmarshalPublic([]byte) error
	Exchange() error
	Public() []byte
	Private() []byte
	Shared() []byte
	Encrypt([]byte) EncryptedMessage
	Decrypt(EncryptedMessage) []byte

	EncryptMessage(Message) EncryptedMessage
	DecryptMessage(EncryptedMessage) Message
}

type ECDHKeyPair struct {
	private *ecdh.PrivateKey
	other   *ecdh.PublicKey
	shared  []byte
	block   cipher.Block
}

func (kp *ECDHKeyPair) Init() error {
	var err error

	kp.other = nil
	kp.shared = make([]byte, 0)
	kp.private, err = ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	return nil
}

func (kp *ECDHKeyPair) MarshalPublic() ([]byte, error) {
	return x509.MarshalPKIXPublicKey(kp.private.PublicKey())
}

func (kp *ECDHKeyPair) UnmarshalPublic(other []byte) error {
	pub, err := x509.ParsePKIXPublicKey(other)
	if err != nil {
		return err
	}
	kp.other = pub.(*ecdh.PublicKey)

	return nil
}

func (kp *ECDHKeyPair) Exchange() error {
	var err error
	kp.shared, err = kp.private.ECDH(kp.other)
	if err != nil {
		return err
	}

	// set the AES block cipher
	kp.block, err = aes.NewCipher(kp.shared)
	if err != nil {
		return err
	}

	return nil
}

func (kp *ECDHKeyPair) Public() []byte {
	return kp.private.PublicKey().Bytes()
}

func (kp *ECDHKeyPair) Private() []byte {
	return kp.private.Bytes()
}

func (kp *ECDHKeyPair) Shared() []byte {
	return kp.private.Bytes()
}

func (kp *ECDHKeyPair) Encrypt(msg []byte) (out EncryptedMessage) {
	out.Iv = make([]byte, aes.BlockSize)
	rand.Read(out.Iv)

	bm := cipher.NewCBCEncrypter(kp.block, out.Iv)
	if diff := len(msg) % kp.block.BlockSize(); diff != 0 {
		for i := 0; i < kp.block.BlockSize()-diff; i++ {
			msg = append(msg, 0)
		}
	}
	out.Emsg = make([]byte, len(msg))
	bm.CryptBlocks(out.Emsg, msg)

	return
}

func (kp *ECDHKeyPair) Decrypt(msg EncryptedMessage) (out []byte) {
	out = make([]byte, len(msg.Emsg))

	bm := cipher.NewCBCDecrypter(kp.block, msg.Iv)
	bm.CryptBlocks(out, msg.Emsg)

	return
}

func (kp *ECDHKeyPair) EncryptMessage(msg Message) EncryptedMessage {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)

	enc.Encode(msg)

	return kp.Encrypt(buf.Bytes())
}

func (kp *ECDHKeyPair) DecryptMessage(msg EncryptedMessage) (out Message) {
	raw := kp.Decrypt(msg)

	buf := bytes.NewBuffer(raw)
	dec := gob.NewDecoder(buf)

	dec.Decode(&out)
	return
}
