package server

import (
	"JanArsMAI/Caller/internal/application/client"
	"JanArsMAI/Caller/internal/application/hub"
	"JanArsMAI/Caller/internal/application/updater"
	"context"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type wsServer struct {
	Updater *websocket.Upgrader
	Hub     *hub.Hub
	Mux     *http.ServeMux
	Srv     *http.Server
}

func NewWsServer(hub *hub.Hub, addr string) *wsServer {
	mux := http.NewServeMux()
	return &wsServer{
		Updater: updater.NewUpdater(),
		Hub:     hub,
		Mux:     mux,
		Srv: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}

func (ws *wsServer) Start() error {
	go ws.Hub.Run()
	ws.Mux.HandleFunc("/", StaticHandler)
	ws.Mux.HandleFunc("/ws", ws.WebSocketHandler)
	return ws.Srv.ListenAndServe()
}

func (ws *wsServer) Stop(ctx context.Context) error {
	return ws.Srv.Shutdown(ctx)
}

func StaticHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func (s *wsServer) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := s.Updater.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Ошибка апгрейда WebSocket:", err)
		return
	}
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = uuid.New().String()
	}
	c := &client.Client{
		ID:        uuid.New().String(),
		Conn:      conn,
		Send:      make(chan []byte, 256),
		Room:      roomID,
		UserAgent: r.UserAgent(),
	}
	s.Hub.Register <- c
	log.Printf("Client with id %s is in room: %s", c.ID, roomID)
	welcomeMsg := map[string]interface{}{
		"type":     "welcome",
		"clientId": c.ID,
		"roomId":   roomID,
	}
	if err := conn.WriteJSON(welcomeMsg); err != nil {
		log.Printf("Error to send welcome: %v", err)
	}
	go c.WritePump()
	go c.ReadPump(s.Hub.BroadcastToRoom)
}
