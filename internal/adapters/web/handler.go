package web

import (
	"encoding/json"
	"marketflow/internal/adapters/redis"
	"marketflow/internal/app/logger"
	"marketflow/internal/models/domain"
	"net/http"
)

type Server struct {
	repo    domain.PriceRepository
	cache   *redis.RedisCache
	manager *mode.Manager
}

func NewServer(repo domain.PriceRepository, cache *redis.RedisCache, manager *mode.Manager) *Server {
	return &Server{
		repo:    repo,
		cache:   cache,
		manager: manager,
	}
}

func (s *Server) handleLowestPrice(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		// "exchange": exchange,
		// "pair":     symbol,
		// "price":    minPrice,
		// "time":     minTime,
	})
}

func (s *Server) handleAveragePrice(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		// "exchange": exchange,
		// "pair":     symbol,
		// "price":    sum / float64(len(stats)),
	})
}

func (s *Server) handleLatestPrice() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			// "exchange": update.Exchange,
			// "pair":     update.Pair,
			// "price":    update.Price,
			// "time":     update.Time,
		})
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, nil)
}

func (s *Server) handleHighestPrice(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		// "exchange": exchange,
		// "pair":     symbol,
		// "price":    maxPrice,
		// "time":     maxTime,
	})
}

func (s *Server) handleSetTestMode(input chan<- domain.PriceUpdate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"mode": "test"})
	}
}

func (s *Server) handleSetLiveMode(input chan<- domain.PriceUpdate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"mode": "live"})
	}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("failed to encode response", "error", err)
	}
}
