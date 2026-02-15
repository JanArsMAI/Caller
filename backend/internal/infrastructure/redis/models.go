package redisrepo

import (
	"encoding/json"
	"time"
)

type ClientInfo struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	JoinedAt  time.Time `json:"joined_at"`
	UserAgent string    `json:"user_agent,omitempty"`
}

type Message struct {
	Type      string    `json:"type"`
	From      string    `json:"from"`
	RoomID    string    `json:"room_id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type RoomStats struct {
	RoomID    string    `json:"room_id"`
	Clients   int64     `json:"clients_count"`
	CreatedAt time.Time `json:"created_at"`
	LastSeen  time.Time `json:"last_seen"`
}

func (m *Message) ToJSON() []byte {
	data, _ := json.Marshal(m)
	return data
}

func (m *Message) FromJson(data []byte) error {
	return json.Unmarshal(data, m)
}
