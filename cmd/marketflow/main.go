package main

import (
	"context"
	"flag"
	"fmt"
	"marketflow/config"
	"marketflow/internal/adapters/cache"
	"marketflow/internal/adapters/postgres"
	"marketflow/internal/app/logger"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// CLI flags
	port := flag.Int("port", 8080, "Port number")
	flag.Parse()

	// Load config and init logger
	cfg := config.Load()
	logger.Init(cfg.AppEnv)

	// Connect to PostgreSQL
	db, err := postgres.NewPostgresDB(cfg)
	if err != nil {
		logger.Error("failed to connect to DB", "err", err)
		return
	}
	defer db.Close()
	logger.Info("connected to PostgreSQL", "host", cfg.DB.Host, "db", cfg.DB.Name)

	// Initialize Redis cache
	cache.InitRedis(cfg.Redis)
	defer cache.CloseRedis()

	// Create basic router
	router := http.NewServeMux()

	// Register routes
	router.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("DB connection error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	router.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("MarketFlow API"))
	})

	// Channel for graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	// WaitGroup to wait for background goroutines
	var wg sync.WaitGroup
	wg.Add(1)

	// Context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background worker for data fetching and saving
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Fetch data from endpoint
				prices, err := cache.FetchDataFromEndpoint()
				if err != nil {
					logger.Error("Error fetching data", "error", err)
					continue
				}

				// Cache data in Redis
				if err := cache.CacheDataInRedis(prices); err != nil {
					logger.Error("Error caching data in Redis", "error", err)
					continue
				}

				// Save data to PostgreSQL
				if err := postgres.SavePrices(db, prices); err != nil {
					logger.Error("Error saving prices to PostgreSQL", "error", err)
					continue
				}

				logger.Info("Successfully processed prices", "count", len(prices))

			case <-ctx.Done():
				logger.Info("Stopping background worker")
				return
			}
		}
	}()

	// Start server in a separate goroutine
	go func() {
		addr := fmt.Sprintf(":%d", *port)
		logger.Info("starting server", "address", addr)

		if err := http.ListenAndServe(addr, router); err != nil {
			logger.Error("server failed", "error", err)
			stopChan <- syscall.SIGTERM // Trigger shutdown
		}
	}()

	// Wait for shutdown signal
	<-stopChan
	logger.Info("Shutting down server...")

	// Cancel context and wait for background goroutines
	cancel()
	wg.Wait()

	logger.Info("Server stopped gracefully")
}
