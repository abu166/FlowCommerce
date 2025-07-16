package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"marketflow/config"
	"marketflow/internal/adapters/cache"
	"marketflow/internal/adapters/postgres"
	"marketflow/internal/app/logger"
	"marketflow/internal/domain/models"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	dataMode     string = "live" // Default to live mode
	dataModeLock sync.RWMutex
)

func main() {
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
	redisClient, err := cache.InitRedis(cfg)
	if err != nil {
		logger.Error("failed to connect to Redis", "err", err)
		return
	}
	defer cache.CloseRedis()

	// Initialize aggregator
	aggregator := postgres.NewAggregator(db, redisClient)
	aggregator.Start()
	defer aggregator.Stop()

	// Create basic router
	router := http.NewServeMux()

	// Register routes
	// router.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
	// 	if err := db.Ping(); err != nil {
	// 		w.WriteHeader(http.StatusServiceUnavailable)
	// 		w.Write([]byte("DB connection error"))
	// 		return
	// 	}
	// 	if _, err := cache.GetRedisClient().Ping(r.Context()).Result(); err != nil {
	// 		w.WriteHeader(http.StatusServiceUnavailable)
	// 		w.Write([]byte("Redis connection error"))
	// 		return
	// 	}
	// 	w.WriteHeader(http.StatusOK)
	// 	w.Write([]byte("OK"))
	// })

	router.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("MarketFlow API"))
	})

	router.HandleFunc("GET /prices/latest/{symbol}", func(w http.ResponseWriter, r *http.Request) {
		symbol := r.PathValue("symbol")
		prices, err := cache.GetAllPrices()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var latest *models.Price
		for _, p := range prices {
			if p.Symbol == symbol && (latest == nil || p.Timestamp > latest.Timestamp) {
				latest = &p
			}
		}
		if latest == nil {
			http.Error(w, "Price not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(latest)
	})

	router.HandleFunc("GET /prices/latest/{exchange}/{symbol}", func(w http.ResponseWriter, r *http.Request) {
		exchange, symbol := r.PathValue("exchange"), r.PathValue("symbol")
		price, err := cache.GetPrice(exchange, symbol)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(price)
	})

	router.HandleFunc("GET /prices/highest/{symbol}", handleHistoricalPrice(db, "max_price"))
	router.HandleFunc("GET /prices/highest/{exchange}/{symbol}", handleHistoricalPrice(db, "max_price"))
	router.HandleFunc("GET /prices/lowest/{symbol}", handleHistoricalPrice(db, "min_price"))
	router.HandleFunc("GET /prices/lowest/{exchange}/{symbol}", handleHistoricalPrice(db, "min_price"))
	router.HandleFunc("GET /prices/average/{symbol}", handleHistoricalPrice(db, "average_price"))
	router.HandleFunc("GET /prices/average/{exchange}/{symbol}", handleHistoricalPrice(db, "average_price"))

	router.HandleFunc("POST /mode/test", func(w http.ResponseWriter, r *http.Request) {
		dataModeLock.Lock()
		dataMode = "test"
		dataModeLock.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Switched to test mode"))
	})

	router.HandleFunc("POST /mode/live", func(w http.ResponseWriter, r *http.Request) {
		dataModeLock.Lock()
		dataMode = "live"
		dataModeLock.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Switched to live mode"))
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

	port := flag.Int("port", 8080, "Port number")
	flag.Parse()

	// Start background worker for data fetching and saving
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		logger.Info("Background worker started")
		exchangeOrder := []string{"exchange1:40101", "exchange2:40102", "exchange3:40103"}
		for {
			select {
			case <-ticker.C:
				logger.Debug("Starting data fetch cycle")
				for _, endpoint := range exchangeOrder {
					var prices []models.Price
					var err error
					dataModeLock.RLock()
					if dataMode == "test" {
						prices, err = fetchMockData(endpoint)
					} else {
						prices, err = cache.FetchDataFromEndpoint(endpoint)
					}
					dataModeLock.RUnlock()
					if err != nil {
						logger.Error("Error fetching data", "error", err)
						continue
					}
					logger.Debug("Fetched prices", "count", len(prices), "first_symbol", prices[0].Symbol)
					fmt.Println(prices)
					if err := cache.CacheDataInRedis(prices); err != nil {
						logger.Error("Error caching data in Redis", "error", err)
						continue
					}
					logger.Debug("Successfully cached prices in Redis")
					logger.Info("Successfully processed prices",
						"count", len(prices),
						"redis_success", true,
						"postgres_success", true)
				}
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
			stopChan <- syscall.SIGTERM
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

// fetchMockData generates mock price data for testing
func fetchMockData(endpoint string) ([]models.Price, error) {
	exchange := strings.Split(endpoint, ":")[0]
	symbols := []string{"BTCUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT", "ETHUSDT"}
	var prices []models.Price
	for _, symbol := range symbols {
		prices = append(prices, models.Price{
			Exchange:  exchange,
			Symbol:    symbol,
			Price:     1000 + rand.Float64()*99000, // Random price between 1000 and 100000
			Timestamp: time.Now().UnixNano() / 1e6,
		})
	}
	return prices, nil
}

// handleHistoricalPrice handles queries for highest, lowest, and average prices
func handleHistoricalPrice(db *sqlx.DB, field string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exchange, symbol := r.PathValue("exchange"), r.PathValue("symbol")
		period := r.URL.Query().Get("period")
		duration, err := parseDuration(period)
		if err != nil {
			http.Error(w, "Invalid period", http.StatusBadRequest)
			return
		}

		query := fmt.Sprintf(`
			SELECT %s
			FROM price_aggregates
			WHERE pair_name = $1
			AND timestamp >= $2
		`, field)
		args := []interface{}{symbol, time.Now().Add(-duration)}
		if exchange != "" {
			query += " AND exchange = $3"
			args = append(args, exchange)
		}
		query += fmt.Sprintf(" ORDER BY %s DESC LIMIT 1", field)

		var price float64
		err = db.GetContext(r.Context(), &price, query, args...)
		if err != nil {
			http.Error(w, "Price not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]float64{field: price})
	}
}

// parseDuration converts a duration string (e.g., "1m", "30s") to time.Duration
func parseDuration(period string) (time.Duration, error) {
	if period == "" {
		period = "1m" // Default period
	}
	return time.ParseDuration(period)
}
