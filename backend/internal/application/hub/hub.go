package hub

import (
	"JanArsMAI/Caller/internal/application/client"
	"JanArsMAI/Caller/internal/infrastructure/livekit"
	redisrepo "JanArsMAI/Caller/internal/infrastructure/redis"
	"context"
	"encoding/json"
	"log"
	"time"
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
	LiveKitCfg  *livekit.LiveKitConfig
	quit        chan struct{}

	redisRepo *redisrepo.RedisRepo
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewHub(cfg *livekit.LiveKitConfig, cl *redisrepo.RedisRepo) *Hub {
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
				log.Printf("Failed to save client to Redis: %v", err)
			}
			h.connections[cl.ID] = cl
			log.Printf("client %s joined room %s", cl.ID, cl.Room)
			h.sendLiveKitToken(cl)

		case cl := <-h.Unregister:
			if err := h.redisRepo.RemoveClient(h.ctx, cl.ID); err != nil {
				log.Printf("Failed to remove client from Redis: %v", err)
			}
			delete(h.connections, cl.ID)
			close(cl.Send)
			log.Printf("client %s left room %s", cl.ID, cl.Room)

		case msg := <-h.Broadcast:
			var originalMsg map[string]any
			if err := json.Unmarshal(msg.Message, &originalMsg); err != nil {
				log.Printf("Failed to parse message: %v", err)
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
				log.Printf("Failed to publish message: %v", err)
			}
			_ = h.redisRepo.SaveMessage(h.ctx, msg.RoomID, redisMsg)

		case <-h.quit:
			log.Println("Stopping hub...")
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
				log.Printf("Failed to parse Redis message: %v", err)
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
						log.Printf("client %s slow, dropping message", cl.ID[:8])
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
		log.Printf("Failed to generate LiveKit token: %v", err)
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
		log.Printf("LiveKit token sent to %s", cl.ID[:8])
	default:
		log.Printf("Client %s slow, dropping token", cl.ID[:8])
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
				log.Printf("Broadcast channel full for room %s", roomID[:8])
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
		log.Printf("Broadcast channel full for room %s", roomID[:8])
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
