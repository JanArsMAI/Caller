package hub

import (
	"JanArsMAI/Caller/internal/application/client"
	"JanArsMAI/Caller/internal/config"

	redisrepo "JanArsMAI/Caller/internal/infrastructure/redis"
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"
)

type BroadcastMsg struct {
	Message  []byte
	RoomID   string
	ClientID string
}

type Hub struct {
	connections map[string]*client.Client
	Register    chan *client.Client
	Unregister  chan *client.Client
	Broadcast   chan BroadcastMsg
	LiveKitCfg  *config.LiveKitConfig
	quit        chan struct{}
	Logger      *zap.Logger

	redisRepo *redisrepo.RedisRepo
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewHub(cfg *config.LiveKitConfig, cl *redisrepo.RedisRepo, lg *zap.Logger) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	return &Hub{
		connections: make(map[string]*client.Client),
		Register:    make(chan *client.Client),
		Unregister:  make(chan *client.Client),
		Broadcast:   make(chan BroadcastMsg, 100),
		LiveKitCfg:  cfg,
		quit:        make(chan struct{}),
		redisRepo:   cl,
		ctx:         ctx,
		cancel:      cancel,
		Logger:      lg,
	}
}

func (h *Hub) Run() {
	go h.listenToRedis()

	for {
		select {
		case cl := <-h.Register:
			info := &redisrepo.ClientInfo{
				ID:        cl.ID,
				RoomID:    cl.Room,
				JoinedAt:  time.Now(),
				UserAgent: cl.UserAgent,
			}

			if err := h.redisRepo.AddClient(h.ctx, info); err != nil {
				h.Logger.Error("Failed to save client to Redis: %v", zap.Error(err))
			}
			h.connections[cl.ID] = cl
			h.Logger.Info("client joined room", zap.String("id", cl.ID), zap.String("room", cl.Room))
			h.sendLiveKitToken(cl)

		case cl := <-h.Unregister:
			if err := h.redisRepo.RemoveClient(h.ctx, cl.ID); err != nil {
				h.Logger.Error("Failed to remove client from Redis: %v", zap.Error(err))
			}
			delete(h.connections, cl.ID)
			close(cl.Send)
			h.Logger.Info("client left room", zap.String("id", cl.ID), zap.String("room", cl.Room))

		case msg := <-h.Broadcast:
			var originalMsg map[string]any
			if err := json.Unmarshal(msg.Message, &originalMsg); err != nil {
				h.Logger.Error("Failed to parse message: %v", zap.Error(err))
				continue
			}

			messageText := ""
			if text, ok := originalMsg["message"].(string); ok {
				messageText = text
			} else if content, ok := originalMsg["content"].(string); ok {
				messageText = content
			} else {
				messageText = string(msg.Message)
			}
			redisMsg := &redisrepo.Message{
				Type:      "chat",
				From:      msg.ClientID,
				RoomID:    msg.RoomID,
				Content:   messageText,
				Timestamp: time.Now(),
			}
			if err := h.redisRepo.PublishMessage(h.ctx, msg.RoomID, redisMsg); err != nil {
				h.Logger.Error("Failed to publish message: %v", zap.Error(err))
			}
			_ = h.redisRepo.SaveMessage(h.ctx, msg.RoomID, redisMsg)

		case <-h.quit:
			h.Logger.Info("Stopping hub...")
			h.cancel()

			for _, cl := range h.connections {
				close(cl.Send)
			}
			return
		}
	}
}
func (h *Hub) listenToRedis() {
	pubsub := h.redisRepo.SubscribeAllRooms(h.ctx)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case msg := <-ch:
			var redisMsg redisrepo.Message
			if err := json.Unmarshal([]byte(msg.Payload), &redisMsg); err != nil {
				h.Logger.Error("Failed to parse Redis message: %v", zap.Error(err))
				continue
			}
			for _, cl := range h.connections {
				if cl.Room == redisMsg.RoomID {
					if cl.ID == redisMsg.From {
						continue
					}

					select {
					case cl.Send <- []byte(msg.Payload):
					default:
						h.Logger.Error("client slow, dropping message", zap.String("id", cl.ID[:8]))
					}
				}
			}

		case <-h.ctx.Done():
			return
		}
	}
}

func (h *Hub) sendLiveKitToken(cl *client.Client) {
	if h.LiveKitCfg == nil {
		return
	}

	token, err := h.LiveKitCfg.GenerateToken(cl.Room, cl.ID)
	if err != nil {
		h.Logger.Error("Failed to generate LiveKit token: %v", zap.Error(err))
		return
	}

	tokenMsg := map[string]any{
		"type":       "livekit-token",
		"token":      token,
		"livekitUrl": h.LiveKitCfg.ApiUrl,
		"room":       cl.Room,
		"identity":   cl.ID,
	}

	tokenData, _ := json.Marshal(tokenMsg)

	select {
	case cl.Send <- tokenData:
		h.Logger.Info("LiveKit token sent to", zap.String("id", cl.ID[:8]))
	default:
		h.Logger.Error("Client slow, dropping token", zap.String("id", cl.ID[:8]))
	}
}

func (h *Hub) BroadcastToRoom(message []byte, roomID string) {
	var msg map[string]any
	if err := json.Unmarshal(message, &msg); err == nil {
		if clientID, ok := msg["from"].(string); ok {
			select {
			case h.Broadcast <- BroadcastMsg{
				RoomID:   roomID,
				Message:  message,
				ClientID: clientID,
			}:
			default:
				h.Logger.Warn("Broadcast channel full for room", zap.String("room", roomID[:8]))
			}
			return
		}
	}
	select {
	case h.Broadcast <- BroadcastMsg{
		RoomID:   roomID,
		Message:  message,
		ClientID: "system",
	}:
	default:
		h.Logger.Warn("Broadcast channel full for room", zap.String("room", roomID[:8]))
	}
}

func (h *Hub) GetRoomClients(ctx context.Context, roomID string) ([]string, error) {
	return h.redisRepo.GetRoomClients(ctx, roomID)
}

func (h *Hub) GetRoomClientsCount(ctx context.Context, roomID string) (int64, error) {
	return h.redisRepo.GetRoomClientsCount(ctx, roomID)
}

func (h *Hub) Stop() {
	close(h.quit)
}
