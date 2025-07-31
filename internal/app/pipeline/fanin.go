package pipeline

import (
	"context"
	"log"
	"marketflow/internal/models/domain"
)

func FanIn(ctx context.Context, clients []domain.ExchangeClient) <-chan domain.PriceUpdate {
	out := make(chan domain.PriceUpdate, 100)

	for _, client := range clients {
		go func(c domain.ExchangeClient) {
			err := c.Start(ctx, out)
			if err != nil {
				log.Printf("[FAN-IN] error starting client: %v", err)
			}
		}(client)
	}

	return out
}
