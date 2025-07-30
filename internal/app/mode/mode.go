package mode

import (
	"context"
	"errors"
	"marketflow/internal/adapters/exchange"
	"marketflow/internal/config"
	"marketflow/internal/domain"
	"marketflow/internal/logger"
	"sync"
)

type Mode string

const (
	Live Mode = "live"
	Test Mode = "test"
)

type Manager struct {
	mu         sync.Mutex
	mode       Mode
	clients    []domain.ExchangeClient
	cancelFunc context.CancelFunc
	cfg        *config.Config
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		mode: Test,
		cfg:  cfg,
	}
}

func (m *Manager) Start(out chan<- domain.PriceUpdate, mode Mode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mode == mode {
		logger.Warn("mode already set, restarting clients", "mode", mode)
		if m.cancelFunc != nil {
			m.cancelFunc()
			for _, client := range m.clients {
				client.Stop()
			}
		}
	} else {
		if m.cancelFunc != nil {
			logger.Info("stopping previous mode")
			m.cancelFunc()
			for _, client := range m.clients {
				client.Stop()
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	m.mode = mode

	m.clients = nil
	switch mode {
	case Test:
		m.clients = []domain.ExchangeClient{
			exchange.NewTestGenerator("ex1"),
			exchange.NewTestGenerator("ex2"),
			exchange.NewTestGenerator("ex3"),
		}
	case Live:
		for _, ex := range m.cfg.Exchanges {
			m.clients = append(m.clients, exchange.NewTCPClient(ctx, ex.Name, ex.Address))
		}
	default:
		return errors.New("invalid mode")
	}

	for _, client := range m.clients {
		go func(c domain.ExchangeClient) {
			if err := c.Start(ctx, out); err != nil {
				logger.Error("failed to start client", "client", c, "error", err)
			}
		}(client)
	}

	logger.Info("started mode", "mode", mode)
	return nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancelFunc != nil {
		logger.Info("stopping mode", "mode", m.mode)
		m.cancelFunc()
		for _, client := range m.clients {
			client.Stop()
		}
		m.cancelFunc = nil
		m.clients = nil
	}
}
