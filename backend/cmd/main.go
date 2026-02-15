package main

import (
	"JanArsMAI/Caller/internal/application/hub"
	"JanArsMAI/Caller/internal/infrastructure/livekit"
	redis "JanArsMAI/Caller/internal/infrastructure/redis"
	redisrepo "JanArsMAI/Caller/internal/infrastructure/redis"
	"JanArsMAI/Caller/internal/presentation/server"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error to load configs:", err)
	}
	lkCfg, err := livekit.NewLiveKitConfig()
	if err != nil {
		log.Fatal("error to load configs:", err)
	} else {
		log.Printf("livekit is ready on: %s", lkCfg.ApiUrl)
	}

	redisClient := redis.NewRedisConnection()
	if redisClient == nil {
		log.Fatal("Failed to Start Redis")
	}

	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Redis is failed Ping Pong Check!")
	}

	repo := redisrepo.NewRedisRepo(redisClient)
	mainHub := hub.NewHub(lkCfg, repo)
	srv := server.NewWsServer(mainHub, "localhost:8080")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Println("Server running on: http://localhost:8080")
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error: %v", err)
		}
	}()

	<-stop

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	mainHub.Stop()

	if err := srv.Stop(ctx); err != nil {
		log.Printf("Ошибка при остановке сервера: %v", err)
	}
	log.Println("Server is finished shutdowning")
}
