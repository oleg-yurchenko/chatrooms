package shared

// UserId is a unique ID for a user upon joining a chatroom
type UserId uint64

type Message struct {
	senderId UserId
	nickname string
	message  string
}
