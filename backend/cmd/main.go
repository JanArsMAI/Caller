package main

import (
	"JanArsMAI/Caller/internal/di"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()
	container, err := di.NewContainer(ctx)
	if err != nil {
		log.Fatal("Failed to initialize application:", err)
	}
	defer container.Close()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		container.Logger.Info("Server starting", zap.String("addr", container.Server.Srv.Addr))
		if err := container.Server.Start(); err != nil && err != http.ErrServerClosed {
			container.Logger.Fatal("Server failed", zap.Error(err))
		}
	}()
	<-stop
	container.Logger.Info("Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	container.Hub.Stop()

	if err := container.Server.Stop(ctx); err != nil {
		container.Logger.Error("Server shutdown error", zap.Error(err))
	}

	container.Logger.Info("Server stopped")
}
