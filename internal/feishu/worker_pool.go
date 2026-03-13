package feishu

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

// Errors returned by WorkerPool
var (
	ErrQueueFull = errors.New("worker queue full")
	ErrClosed    = errors.New("worker pool closed")
)

// Job represents a unit of work to be processed by the worker pool
type Job struct {
	EventID string       // Event identifier for logging
	Handler func() error // Handler function that processes the job
}

// WorkerPool manages a pool of workers that process jobs from a bounded queue.
// It supports panic recovery, graceful shutdown, and non-blocking job submission.
type WorkerPool struct {
	workers  int
	jobQueue chan Job
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.RWMutex // Protects closed state
	closed   bool         // Prevents send on closed channel
}

// NewWorkerPool creates a new WorkerPool with the specified number of workers
// and queue size.
//
// workers: number of concurrent worker goroutines
// queueSize: maximum number of pending jobs in the queue
func NewWorkerPool(workers, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		workers:  workers,
		jobQueue: make(chan Job, queueSize),
		ctx:      ctx,
		cancel:   cancel,
		closed:   false,
	}
}

// Start launches the worker goroutines. Each worker processes jobs from the
// queue until the pool is shut down.
func (p *WorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go func(workerID int) {
			defer p.wg.Done()
			for {
				select {
				case job, ok := <-p.jobQueue:
					if !ok {
						return // Channel closed, worker exits
					}
					// Panic recovery wraps each job execution, not the goroutine.
					// This ensures the worker continues processing after a panic.
					p.executeJob(workerID, job)
				case <-p.ctx.Done():
					return // Context cancelled, worker exits
				}
			}
		}(i)
	}
}

// executeJob runs a single job with panic recovery.
// Panics are logged but do not crash the worker.
func (p *WorkerPool) executeJob(workerID int, job Job) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Worker %d] panic recovered for job %s: %v", workerID, job.EventID, r)
		}
	}()

	if err := job.Handler(); err != nil {
		log.Printf("[Worker %d] job %s error: %v", workerID, job.EventID, err)
	}
}

// Submit adds a job to the queue. It returns immediately with an error if the
// queue is full or the pool is closed.
//
// Returns:
//   - nil: job was successfully queued
//   - ErrQueueFull: queue is at capacity
//   - ErrClosed: pool has been shut down
func (p *WorkerPool) Submit(job Job) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return ErrClosed
	}

	select {
	case p.jobQueue <- job:
		return nil
	default:
		return ErrQueueFull
	}
}

// Shutdown gracefully stops the worker pool. It prevents new job submissions,
// waits for pending jobs to complete, then exits all workers.
//
// If workers don't finish within the timeout, the context is cancelled to
// force an exit, and an error is returned.
func (p *WorkerPool) Shutdown(timeout time.Duration) error {
	// Acquire write lock to prevent concurrent Submit during close
	p.mu.Lock()
	p.closed = true
	close(p.jobQueue)
	p.mu.Unlock()

	// Wait for all workers to complete
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		p.cancel() // Force cancel context
		return errors.New("shutdown timeout")
	}
}

// QueueLen returns the current number of jobs waiting in the queue.
// Useful for metrics and monitoring.
func (p *WorkerPool) QueueLen() int {
	return len(p.jobQueue)
}
