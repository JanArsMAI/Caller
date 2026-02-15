package redisrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRepo struct {
	db   *redis.Client
	keys Keys
}

func NewRedisRepo(db *redis.Client) *RedisRepo {
	return &RedisRepo{
		db:   db,
		keys: Keys{},
	}
}

func (r *RedisRepo) AddClient(ctx context.Context, info *ClientInfo) error {
	pipe := r.db.Pipeline()
	pipe.Set(ctx, r.keys.ClientKey(info.ID), info.RoomID, 24*time.Hour)
	pipe.SAdd(ctx, r.keys.RoomClientsKey(info.RoomID), info.ID)
	pipe.HSet(ctx, r.keys.ClientMetaKey(info.ID), map[string]any{
		"joined_at":  info.JoinedAt.Unix(),
		"user_agent": info.UserAgent,
	})
	pipe.Expire(ctx, r.keys.ClientMetaKey(info.ID), 24*time.Hour)
	pipe.HSet(ctx, r.keys.RoomMetaKey(info.RoomID), "last_seen", time.Now().Unix())
	pipe.HSetNX(ctx, r.keys.RoomMetaKey(info.RoomID), "created_at", time.Now().Unix())
	pipe.SAdd(ctx, r.keys.ActiveRoomsKey(), info.RoomID)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisRepo) RemoveClient(ctx context.Context, clientID string) error {
	roomID, err := r.db.Get(ctx, r.keys.ClientKey(clientID)).Result()
	if err == redis.Nil {
		return ErrClientNotFound
	}
	if err != nil {
		return err
	}

	pipe := r.db.Pipeline()
	pipe.SRem(ctx, r.keys.RoomClientsKey(roomID), clientID)
	pipe.Del(ctx, r.keys.ClientKey(clientID))
	pipe.Del(ctx, r.keys.ClientMetaKey(clientID))
	count, _ := r.db.SCard(ctx, r.keys.RoomClientsKey(roomID)).Result()
	if count == 0 {
		pipe.Del(ctx, r.keys.RoomClientsKey(roomID))
		pipe.Del(ctx, r.keys.RoomMetaKey(roomID))
		pipe.SRem(ctx, r.keys.ActiveRoomsKey(), roomID)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisRepo) GetClientRoom(ctx context.Context, clientID string) (string, error) {
	roomID, err := r.db.Get(ctx, r.keys.ClientKey(clientID)).Result()
	if err == redis.Nil {
		return "", ErrClientNotFound
	}
	return roomID, err
}

func (r *RedisRepo) GetClientInfo(ctx context.Context, clientID string) (*ClientInfo, error) {
	roomID, err := r.GetClientRoom(ctx, clientID)
	if err != nil {
		return nil, err
	}
	meta, err := r.db.HGetAll(ctx, r.keys.ClientMetaKey(clientID)).Result()
	if err != nil {
		return nil, err
	}

	joinedAt, _ := meta["joined_at"]
	joinedTime := time.Unix(atol(joinedAt), 0)

	return &ClientInfo{
		ID:        clientID,
		RoomID:    roomID,
		JoinedAt:  joinedTime,
		UserAgent: meta["user_agent"],
	}, nil
}
func (r *RedisRepo) ClientExists(ctx context.Context, clientID string) (bool, error) {
	exists, err := r.db.Exists(ctx, r.keys.ClientKey(clientID)).Result()
	return exists == 1, err
}

func (r *RedisRepo) GetRoomClients(ctx context.Context, roomID string) ([]string, error) {
	return r.db.SMembers(ctx, r.keys.RoomClientsKey(roomID)).Result()
}

func (r *RedisRepo) GetRoomClientsCount(ctx context.Context, roomID string) (int64, error) {
	return r.db.SCard(ctx, r.keys.RoomClientsKey(roomID)).Result()
}

func (r *RedisRepo) IsClientInRoom(ctx context.Context, clientID, roomID string) (bool, error) {
	return r.db.SIsMember(ctx, r.keys.RoomClientsKey(roomID), clientID).Result()
}
func (r *RedisRepo) GetActiveRooms(ctx context.Context) ([]string, error) {
	return r.db.SMembers(ctx, r.keys.ActiveRoomsKey()).Result()
}

func (r *RedisRepo) GetRoomStats(ctx context.Context, roomID string) (*RoomStats, error) {
	count, err := r.GetRoomClientsCount(ctx, roomID)
	if err != nil {
		return nil, err
	}

	meta, err := r.db.HGetAll(ctx, r.keys.RoomMetaKey(roomID)).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	stats := &RoomStats{
		RoomID:  roomID,
		Clients: count,
	}

	if createdAt, ok := meta["created_at"]; ok {
		stats.CreatedAt = time.Unix(atol(createdAt), 0)
	}
	if lastSeen, ok := meta["last_seen"]; ok {
		stats.LastSeen = time.Unix(atol(lastSeen), 0)
	}

	return stats, nil
}
func (r *RedisRepo) RoomExists(ctx context.Context, roomID string) (bool, error) {
	exists, err := r.db.Exists(ctx, r.keys.RoomClientsKey(roomID)).Result()
	return exists == 1, err
}

func (r *RedisRepo) PublishMessage(ctx context.Context, roomID string, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return r.db.Publish(ctx, r.keys.RoomChannel(roomID), data).Err()
}

func (r *RedisRepo) SubscribeRoom(ctx context.Context, roomID string) *redis.PubSub {
	return r.db.Subscribe(ctx, r.keys.RoomChannel(roomID))
}

func (r *RedisRepo) SubscribeAllRooms(ctx context.Context) *redis.PubSub {
	return r.db.PSubscribe(ctx, r.keys.AllRoomsPattern())
}

func (r *RedisRepo) SaveMessage(ctx context.Context, roomID string, msg *Message) error {
	key := r.keys.RoomMessagesKey(roomID)
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	pipe := r.db.Pipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, 99)
	pipe.Expire(ctx, key, 24*time.Hour)

	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisRepo) GetRecentMessages(ctx context.Context, roomID string, limit int64) ([]*Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	data, err := r.db.LRange(ctx, r.keys.RoomMessagesKey(roomID), 0, limit-1).Result()
	if err != nil {
		return nil, err
	}

	messages := make([]*Message, 0, len(data))
	for _, item := range data {
		var msg Message
		if err := json.Unmarshal([]byte(item), &msg); err == nil {
			messages = append(messages, &msg)
		}
	}

	return messages, nil
}

func (r *RedisRepo) ClearRoom(ctx context.Context, roomID string) error {
	clients, err := r.GetRoomClients(ctx, roomID)
	if err != nil {
		return err
	}

	pipe := r.db.Pipeline()
	for _, clientID := range clients {
		pipe.Del(ctx, r.keys.ClientKey(clientID))
		pipe.Del(ctx, r.keys.ClientMetaKey(clientID))
	}
	pipe.Del(ctx, r.keys.RoomClientsKey(roomID))
	pipe.Del(ctx, r.keys.RoomMetaKey(roomID))
	pipe.Del(ctx, r.keys.RoomMessagesKey(roomID))
	pipe.SRem(ctx, r.keys.ActiveRoomsKey(), roomID)

	_, err = pipe.Exec(ctx)
	return err
}
func (r *RedisRepo) GetAllStats(ctx context.Context) ([]*RoomStats, error) {
	rooms, err := r.GetActiveRooms(ctx)
	if err != nil {
		return nil, err
	}

	stats := make([]*RoomStats, 0, len(rooms))
	for _, roomID := range rooms {
		if stat, err := r.GetRoomStats(ctx, roomID); err == nil {
			stats = append(stats, stat)
		}
	}

	return stats, nil
}

func (r *RedisRepo) HealthCheck(ctx context.Context) error {
	return r.db.Ping(ctx).Err()
}
func (r *RedisRepo) Close() error {
	return r.db.Close()
}

func atol(s string) int64 {
	var i int64
	fmt.Sscanf(s, "%d", &i)
	return i
}
