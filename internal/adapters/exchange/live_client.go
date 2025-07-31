package exchange

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"marketflow/internal/models/domain"
	"marketflow/internal/app/logger"
)

type LiveClient struct {
	ctx      context.Context
	exchange string
	addr     string
	conn     net.Conn
	stopCh   chan struct{}
}

func NewTCPClient(ctx context.Context, exchange, addr string) *LiveClient {
	return &LiveClient{
		ctx:      ctx,
		exchange: exchange,
		addr:     addr,
		stopCh:   make(chan struct{}),
	}
}

func (c *LiveClient) Start(ctx context.Context, out chan<- domain.PriceUpdate) error {
	logger.Info("starting live client", "exchange", c.exchange, "addr", c.addr)

	for {
		select {
		case <-ctx.Done():
			logger.Info("live client stopped by context", "exchange", c.exchange)
			return ctx.Err()
		case <-c.stopCh:
			logger.Info("live client stopped", "exchange", c.exchange)
			return nil
		default:
			if err := c.connectAndRead(ctx, out); err != nil {
				logger.Error("connection error", "exchange", c.exchange, "addr", c.addr, "error", err)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-c.stopCh:
					return nil
				case <-time.After(5 * time.Second):
					logger.Info("reconnecting", "exchange", c.exchange, "addr", c.addr)
					continue
				}
			}
		}
	}
}

func (c *LiveClient) connectAndRead(ctx context.Context, out chan<- domain.PriceUpdate) error {
	conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %w", c.addr, err)
	}
	c.conn = conn
	defer conn.Close()

	logger.Info("connected to exchange", "exchange", c.exchange, "addr", c.addr)

	reader := bufio.NewReader(conn)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopCh:
			return nil
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read from %s: %w", c.addr, err)
			}

			var update domain.PriceUpdate
			if err := json.Unmarshal([]byte(line), &update); err != nil {
				logger.Error("failed to unmarshal price update", "exchange", c.exchange, "data", line, "error", err)
				continue
			}
			update.Exchange = c.exchange

			select {
			case out <- update:
				logger.Debug("sent live price update", "exchange", c.exchange, "pair", update.Pair, "price", update.Price)
			case <-ctx.Done():
				return ctx.Err()
			case <-c.stopCh:
				return nil
			}
		}
	}
}

func (c *LiveClient) Stop() error {
	logger.Info("stopping live client", "exchange", c.exchange)
	close(c.stopCh)
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			logger.Error("failed to close connection", "exchange", c.exchange, "error", err)
		}
	}
	return nil
}
