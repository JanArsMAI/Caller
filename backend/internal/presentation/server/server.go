package server

import (
	"JanArsMAI/Caller/internal/application/client"
	"JanArsMAI/Caller/internal/application/hub"
	"JanArsMAI/Caller/internal/application/updater"
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type WsServer struct {
	Updater *websocket.Upgrader
	Hub     *hub.Hub
	Mux     *http.ServeMux
	Srv     *http.Server
	Logger  *zap.Logger
}

func NewWsServer(hub *hub.Hub, addr string, lg *zap.Logger) *WsServer {
	mux := http.NewServeMux()
	return &WsServer{
		Updater: updater.NewUpdater(),
		Hub:     hub,
		Mux:     mux,
		Srv: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		Logger: lg,
	}
}

func (ws *WsServer) Start() error {
	go ws.Hub.Run()
	ws.Mux.HandleFunc("/", StaticHandler)
	ws.Mux.HandleFunc("/ws", ws.WebSocketHandler)
	return ws.Srv.ListenAndServe()
}

func (ws *WsServer) Stop(ctx context.Context) error {
	return ws.Srv.Shutdown(ctx)
}

func StaticHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func (s *WsServer) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := s.Updater.Upgrade(w, r, nil)
	if err != nil {
		s.Logger.Error("Ошибка апгрейда WebSocket:", zap.Error(err))
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
		Logger:    s.Logger,
	}
	s.Hub.Register <- c
	s.Logger.Info("Client with id is in room:", zap.String("id", c.ID), zap.String("room_id", roomID))
	welcomeMsg := map[string]any{
		"type":     "welcome",
		"clientId": c.ID,
		"roomId":   roomID,
	}
	if err := conn.WriteJSON(welcomeMsg); err != nil {
		s.Logger.Error("Error to send welcome: %v", zap.Error(err))
	}
	go c.WritePump()
	go c.ReadPump(s.Hub.BroadcastToRoom)
}
