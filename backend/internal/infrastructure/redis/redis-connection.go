package redisrepo

import (
	"JanArsMAI/Caller/internal/config"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func NewRedisConnection(cfg *config.RedisConfig) *redis.Client {
	if cfg.Host == "" {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}
