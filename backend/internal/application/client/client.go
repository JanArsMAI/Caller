package client

import (
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Client struct {
	ID        string
	Conn      *websocket.Conn
	Send      chan []byte
	Room      string
	UserAgent string
	Logger    *zap.Logger
}

func (c *Client) ReadPump(broadcast func([]byte, string)) {
	defer func() {
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.Logger.Error("error: %v", zap.Error(err))
			}
			break
		}
		broadcast(message, c.Room)
	}
}

func (c *Client) WritePump() {
	for message := range c.Send {
		err := c.Conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
}
