package main

import (
	"context"
	"fmt"
	"log"
	"marketflow/config"
	"marketflow/internal/adapters/redis"
	"marketflow/internal/adapters/storage/postgres"
	"marketflow/internal/adapters/web"
	"marketflow/internal/app/aggregator"
	"marketflow/internal/app/logger"
	"marketflow/internal/app/mode"
	"marketflow/internal/app/pipeline"
	"marketflow/internal/models/domain"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	logger.Init(env)

	logger.Info("starting application", "env", env)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	repo, err := postgres.NewPostgresDB(cfg)
	if err != nil {
		logger.Error("failed to connect to DB", "err", err)
		return
	}
	defer repo.Close()

	redisAddr := fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)
	cache := redis.InitRedis(redisAddr, cfg.Redis.Password, cfg.Redis.DB, cfg.RedisTTL)
	defer cache.Close()

	inputChan := make(chan domain.PriceUpdate, 1000)
	outputChan := make(chan domain.PriceUpdate, 1000)

	manager := mode.NewManager(cfg)
	agg := aggregator.NewAggregator(outputChan, repo, cache, cfg.AggregatorWindow)

	go agg.Start(context.Background())

	for i := 0; i < 5; i++ {
		worker := &pipeline.Worker{
			ID:     i,
			Input:  inputChan,
			Cache:  cache,
			Output: outputChan,
		}
		worker.Start(context.Background())
	}
	if err := manager.Start(inputChan, mode.Test); err != nil {
		if err.Error() == "mode already set" {
			logger.Warn("initial mode already set, continuing")
		} else {
			log.Fatalf("failed to start test mode: %v", err)
		}
	}

	apiServer := web.NewServer(repo, cache, manager)

	srv := &http.Server{
		Addr:    cfg.APIAddr,
		Handler: apiServer.Router(inputChan), // создадим метод Router() чуть ниже
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server failed", "error", err)
			log.Fatalf("API server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	logger.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("API shutdown error", "error", err)
	}
	logger.Info("shutdown complete")
}
