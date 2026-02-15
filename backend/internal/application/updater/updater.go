package updater

import (
	"net/http"

	"github.com/gorilla/websocket"
)

func NewUpdater() *websocket.Upgrader {
	return &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "http://localhost:5173" ||
				origin == "http://localhost:8080" ||
				origin == ""
		},
	}
}
