package shared

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/x509"
	mrand "math/rand/v2"
)

// UserId is a unique ID for a user upon joining a chatroom
type UserId uint64

func MakeUserId() UserId {
	return UserId(mrand.Uint64())
}

type Message struct {
	senderId UserId
	nickname string
	message  string
}

type EncryptedMessage struct {
	iv   []byte
	emsg []byte // encrypted `Message` struct
}

type ChatCommand uint8

const (
	ListOnline ChatCommand = iota
	Whisper
	CopyLast
)

type KeyPair interface {
	Init() error
	// reads own public key stored in the type
	MarshalPublic() ([]byte, error)
	// writes to other public key stored in the type
	UnmarshalPublic([]byte) error
	Exchange([]byte) error
	Public() []byte
	Private() []byte
	Shared() []byte
	Encrypt([]byte) EncryptedMessage
	Decrypt(EncryptedMessage) []byte
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

func (kp *ECDHKeyPair) Exchange(other []byte) error {
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
	out.iv = make([]byte, kp.block.BlockSize())
	rand.Read(out.iv)

	bm := cipher.NewCBCEncrypter(kp.block, out.iv)
	out.emsg = make([]byte, len(msg))
	bm.CryptBlocks(out.emsg, msg)

	return
}

func (kp *ECDHKeyPair) Decrypt(msg EncryptedMessage) (out []byte) {
	out = make([]byte, len(msg.emsg))

	bm := cipher.NewCBCDecrypter(kp.block, msg.iv)
	bm.CryptBlocks(out, msg.emsg)

	return
}
