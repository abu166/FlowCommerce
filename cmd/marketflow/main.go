package main

import (
	"flag"
	"fmt"
	"marketflow/config"
	"marketflow/internal/adapters/postgres"
	"marketflow/internal/app/logger"
	"net/http"
	"os"
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

	
	// Create basic router
	router := http.NewServeMux()

	// Register routes
	router.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	router.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("MarketFlow API"))
	})

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	logger.Info("starting server", "address", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
