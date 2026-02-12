package scheduler

import (
	"context"
	"log"
	"time"
)

type Scheduler struct {
	interval time.Duration
	stopCh   chan struct{}
}

func New(interval time.Duration) *Scheduler {
	return &Scheduler{
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	log.Printf("scheduler: checking for updates every %s", s.interval)

	for {
		select {
		case <-ticker.C:
			log.Println("scheduler: version check triggered (not yet implemented)")
		case <-ctx.Done():
			log.Println("scheduler: stopped")
			return
		case <-s.stopCh:
			log.Println("scheduler: stopped")
			return
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
}
