package redisrepo

import (
	"os"

	"github.com/redis/go-redis/v9"
)

func NewRedisConnection() *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})
}
