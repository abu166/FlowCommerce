package pipeline

import (
	"marketflow/internal/models/domain"
)

func FanOut(in <-chan domain.PriceUpdate, workerCount int) []chan domain.PriceUpdate {
	workerChans := make([]chan domain.PriceUpdate, workerCount)
	for i := range workerChans {
		workerChans[i] = make(chan domain.PriceUpdate, 100)
	}

	go func() {
		i := 0
		for update := range in {
			workerChans[i%workerCount] <- update
			i++
		}
		for _, ch := range workerChans {
			close(ch)
		}
	}()

	return workerChans
}
