package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"marketflow/config"
	"marketflow/internal/adapters/cache"
	"marketflow/internal/adapters/postgres"
	"marketflow/internal/app/logger"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// CLI flags


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
	cache.InitRedis(cfg)
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

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := net.Dial("tcp", "exchange:40101")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer conn.Close()

		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, conn)
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


	conn, _ := net.Dial("tcp", "exchange:40101")
    conn.Write([]byte("TEST\n"))
    buf := make([]byte, 1024)
    n, _ := conn.Read(buf)
    fmt.Println("Received:", string(buf[:n]))
	
	port := flag.Int("port", 8080, "Port number")
	flag.Parse()

	// Start background worker for data fetching and saving
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		logger.Info("Background worker started")

		for {
			select {
			case <-ticker.C:
				logger.Debug("Starting data fetch cycle")

				// Fetch data from endpoint
				prices, err := cache.FetchDataFromEndpoint()
				if err != nil {
					logger.Error("Error fetching data", "error", err)
					continue
				}
				logger.Debug("Fetched prices", "count", len(prices), "first_symbol", prices[0].Symbol)

				// Cache data in Redis
				if err := cache.CacheDataInRedis(prices); err != nil {
					logger.Error("Error caching data in Redis", "error", err)
					continue
				}
				logger.Debug("Successfully cached prices in Redis")

				// Save data to PostgreSQL
				if err := postgres.SavePrices(db, prices); err != nil {
					logger.Error("Error saving prices to PostgreSQL", "error", err, "prices", prices)
					continue
				}
				logger.Info("Successfully processed prices",
					"count", len(prices),
					"redis_success", true,
					"postgres_success", true)

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
