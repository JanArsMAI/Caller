package di

import (
	"JanArsMAI/Caller/internal/application/hub"
	"JanArsMAI/Caller/internal/config"
	redisrepo "JanArsMAI/Caller/internal/infrastructure/redis"
	"JanArsMAI/Caller/internal/logger"
	"JanArsMAI/Caller/internal/presentation/server"
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Container struct {
	Config      *config.Config
	Logger      *zap.Logger
	RedisClient *redis.Client
	RedisRepo   *redisrepo.RedisRepo
	Hub         *hub.Hub
	Server      *server.WsServer
}

func NewContainer(ctx context.Context) (*Container, error) {
	c := &Container{}
	cfg, err := config.LoadFromFile("config.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	c.Config = cfg
	zapLogger := logger.NewLogger(cfg.LoggerConfig.Level)
	c.Logger = zapLogger
	redisClient := redisrepo.NewRedisConnection(&cfg.RedisCfg)
	if redisClient == nil {
		return nil, fmt.Errorf("failed to create Redis connection")
	}
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	c.RedisClient = redisClient
	c.Logger.Info("Connected to Redis")
	c.RedisRepo = redisrepo.NewRedisRepo(redisClient)

	c.Hub = hub.NewHub(&cfg.LiveKitCfg, c.RedisRepo, c.Logger)

	srvDsn := fmt.Sprintf("%s:%s", cfg.ServerCfg.Host, cfg.ServerCfg.Port)
	c.Server = server.NewWsServer(c.Hub, srvDsn, c.Logger)

	return c, nil
}

func (c *Container) Close() error {
	if c.RedisClient != nil {
		if err := c.RedisClient.Close(); err != nil {
			c.Logger.Error("Failed to close Redis", zap.Error(err))
		}
	}
	if c.Logger != nil {
		c.Logger.Sync()
	}
	return nil
}
