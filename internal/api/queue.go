package api

import (
	"sync"
	"time"
)

type QueueStatus struct {
	Busy      bool      `json:"busy"`
	Operation string    `json:"operation,omitempty"`
	AppName   string    `json:"appname,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
}

type OperationQueue struct {
	mu     sync.Mutex
	cliMu  sync.Mutex
	status QueueStatus
}

func NewOperationQueue() *OperationQueue {
	return &OperationQueue{}
}

func (q *OperationQueue) TryStart(operation, appname string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.status.Busy {
		return false
	}

	q.status = QueueStatus{
		Busy:      true,
		Operation: operation,
		AppName:   appname,
		StartedAt: time.Now(),
	}
	return true
}

func (q *OperationQueue) Finish() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.status = QueueStatus{}
}

func (q *OperationQueue) Status() QueueStatus {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.status
}

func (q *OperationQueue) WithCLI(fn func() error) error {
	q.cliMu.Lock()
	defer q.cliMu.Unlock()
	return fn()
}
