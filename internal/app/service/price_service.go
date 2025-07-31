package service

import (
	"context"
	"time"

	"marketflow/internal/app/aggregator"
	"marketflow/internal/app/pipeline"
	"marketflow/internal/models/domain"
	"marketflow/internal/app/logger"
)

type PriceService struct {
	input       <-chan domain.PriceUpdate
	repo        domain.PriceRepository
	cache       domain.Cache
	aggregator  chan domain.PriceUpdate
	aggregSvc   *aggregator.Aggregator
	workers     []*pipeline.Worker
	numWorkers  int
	aggInterval string
}

func NewPriceService(
	input <-chan domain.PriceUpdate,
	repo domain.PriceRepository,
	cache domain.Cache,
	numWorkers int,
	aggInterval string,
) *PriceService {
	return &PriceService{
		input:       input,
		repo:        repo,
		cache:       cache,
		aggregator:  make(chan domain.PriceUpdate, 100),
		numWorkers:  numWorkers,
		aggInterval: aggInterval,
	}
}

func (s *PriceService) Start(ctx context.Context) {
	logger.Info("starting price service with worker pool", "workers", s.numWorkers)

	workerChans := pipeline.FanOut(s.input, s.numWorkers)

	s.workers = make([]*pipeline.Worker, s.numWorkers)
	for i := 0; i < s.numWorkers; i++ {
		s.workers[i] = &pipeline.Worker{
			ID:     i,
			Input:  workerChans[i],
			Cache:  s.cache,
			Output: s.aggregator,
		}
		s.workers[i].Start(ctx)
		logger.Info("started worker", "worker_id", i)
	}

	duration, err := time.ParseDuration(s.aggInterval)
	if err != nil {
		logger.Error("invalid aggregation interval, using default 1m", "interval", s.aggInterval, "error", err)
		duration = time.Minute
	}

	s.aggregSvc = aggregator.NewAggregator(s.aggregator, s.repo, s.cache, duration)
	go s.aggregSvc.Start(ctx)
	logger.Info("started aggregator service", "interval", duration)

	logger.Info("price service started")
}

func (s *PriceService) Stop() {
	logger.Info("stopping price service")
	close(s.aggregator)
}
