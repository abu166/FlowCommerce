package pipeline

import (
	"context"
	"marketflow/internal/app/logger"
	"marketflow/internal/models/domain"
)

type Worker struct {
	ID     int
	Input  <-chan domain.PriceUpdate
	Cache  domain.Cache
	Output chan<- domain.PriceUpdate
}

func (w *Worker) Start(ctx context.Context) {
	go func() {
		for update := range w.Input {
			err := w.Cache.SetLatest(ctx, update)
			if err != nil {
				logger.Error("cache error", "worker", w.ID, "error", err)
			}

			if w.Output != nil {
				w.Output <- update
			}
		}
	}()
}
