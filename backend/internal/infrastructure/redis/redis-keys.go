package redisrepo

import "fmt"

type Keys struct{}

func (k *Keys) ClientKey(clientID string) string {
	return fmt.Sprintf("client:%s", clientID)
}

func (k *Keys) ClientMetaKey(clientID string) string {
	return fmt.Sprintf("client:%s:meta", clientID)
}

func (k *Keys) RoomClientsKey(roomID string) string {
	return fmt.Sprintf("room:%s:clients", roomID)
}

func (k *Keys) RoomMetaKey(roomID string) string {
	return fmt.Sprintf("room:%s:meta", roomID)
}

func (k *Keys) RoomMessagesKey(roomID string) string {
	return fmt.Sprintf("room:%s:messages", roomID)
}

func (k *Keys) RoomChannel(roomID string) string {
	return fmt.Sprintf("room:%s", roomID)
}

func (k *Keys) ActiveRoomsKey() string {
	return "rooms:active"
}

func (k *Keys) AllRoomsPattern() string {
	return "room:*"
}
