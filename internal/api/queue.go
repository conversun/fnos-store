package api

import (
	"sync"
	"time"
)

type activeOp struct {
	Operation string
	StartedAt time.Time
}

type QueueStatus struct {
	Busy      bool      `json:"busy"`
	Operation string    `json:"operation,omitempty"`
	AppName   string    `json:"appname,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
}

type OperationQueue struct {
	mu               sync.Mutex
	cliMu            sync.Mutex
	activeOps        map[string]*activeOp
	selfUpdateActive bool
}

func NewOperationQueue() *OperationQueue {
	return &OperationQueue{
		activeOps: make(map[string]*activeOp),
	}
}

func (q *OperationQueue) TryStart(operation, appname string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.selfUpdateActive {
		return false
	}

	if _, exists := q.activeOps[appname]; exists {
		return false
	}

	q.activeOps[appname] = &activeOp{
		Operation: operation,
		StartedAt: time.Now(),
	}
	return true
}

// TryStartExclusive rejects if any operations are active; sets global exclusive mode.
func (q *OperationQueue) TryStartExclusive(operation, appname string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.activeOps) > 0 {
		return false
	}

	q.selfUpdateActive = true
	q.activeOps[appname] = &activeOp{
		Operation: operation,
		StartedAt: time.Now(),
	}
	return true
}

// Finish clears all active operations. Task 3 migrates call sites to FinishApp.
func (q *OperationQueue) Finish() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.activeOps = make(map[string]*activeOp)
	q.selfUpdateActive = false
}

func (q *OperationQueue) FinishApp(appname string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.activeOps, appname)
}

func (q *OperationQueue) FinishExclusive(appname string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.activeOps, appname)
	q.selfUpdateActive = false
}

// Status returns first active op. Task 3 migrates call sites to ActiveOps.
func (q *OperationQueue) Status() QueueStatus {
	q.mu.Lock()
	defer q.mu.Unlock()
	for appname, op := range q.activeOps {
		return QueueStatus{
			Busy:      true,
			Operation: op.Operation,
			AppName:   appname,
			StartedAt: op.StartedAt,
		}
	}
	return QueueStatus{}
}

func (q *OperationQueue) ActiveOps() []QueueStatus {
	q.mu.Lock()
	defer q.mu.Unlock()

	ops := make([]QueueStatus, 0, len(q.activeOps))
	for appname, op := range q.activeOps {
		ops = append(ops, QueueStatus{
			Busy:      true,
			Operation: op.Operation,
			AppName:   appname,
			StartedAt: op.StartedAt,
		})
	}
	return ops
}

func (q *OperationQueue) IsBusy() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.activeOps) > 0 || q.selfUpdateActive
}

func (q *OperationQueue) WithCLI(fn func() error) error {
	q.cliMu.Lock()
	defer q.cliMu.Unlock()
	return fn()
}
