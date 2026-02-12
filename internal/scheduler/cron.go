package scheduler

import (
	"context"
	"log"
	"sync"
	"time"
)

type CheckFunc func(ctx context.Context) error

type Scheduler struct {
	interval    time.Duration
	checkFn     CheckFunc
	lastCheckFn func() time.Time
	stopCh      chan struct{}

	mu      sync.Mutex
	running bool
	ticker  *time.Ticker
}

func New(interval time.Duration, checkFn CheckFunc, lastCheckFn func() time.Time) *Scheduler {
	return &Scheduler{
		interval:    interval,
		checkFn:     checkFn,
		lastCheckFn: lastCheckFn,
		stopCh:      make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	log.Printf("scheduler: checking every %s", s.interval)

	if s.lastCheckFn != nil {
		lastCheck := s.lastCheckFn()
		if !lastCheck.IsZero() && time.Since(lastCheck) > s.interval {
			log.Println("scheduler: stale check detected, triggering immediate refresh")
			s.runCheck(ctx)
		}
	}

	s.mu.Lock()
	s.ticker = time.NewTicker(s.interval)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.ticker != nil {
			s.ticker.Stop()
		}
		s.running = false
		s.mu.Unlock()
	}()

	for {
		select {
		case <-s.ticker.C:
			s.runCheck(ctx)
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
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

func (s *Scheduler) SetInterval(d time.Duration) {
	if d < 1*time.Hour {
		d = 1 * time.Hour
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.interval = d
	if s.ticker != nil {
		s.ticker.Reset(d)
	}
	log.Printf("scheduler: interval updated to %s", d)
}

func (s *Scheduler) runCheck(ctx context.Context) {
	if s.checkFn == nil {
		return
	}
	log.Println("scheduler: version check triggered")
	if err := s.checkFn(ctx); err != nil {
		log.Printf("scheduler: check failed: %v", err)
	} else {
		log.Println("scheduler: check completed")
	}
}
