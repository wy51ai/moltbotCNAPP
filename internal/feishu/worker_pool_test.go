package feishu

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPool_Submit(t *testing.T) {
	pool := NewWorkerPool(2, 10)
	pool.Start()
	defer pool.Shutdown(time.Second)

	// Track job execution
	var executed int32
	done := make(chan struct{})

	err := pool.Submit(Job{
		EventID: "test-event-1",
		Handler: func() error {
			atomic.AddInt32(&executed, 1)
			close(done)
			return nil
		},
	})

	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Wait for job to complete
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Job was not executed within timeout")
	}

	if atomic.LoadInt32(&executed) != 1 {
		t.Errorf("Expected 1 job executed, got %d", executed)
	}
}

func TestWorkerPool_Submit_HandlerError(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()
	defer pool.Shutdown(time.Second)

	done := make(chan struct{})
	expectedErr := errors.New("test error")

	err := pool.Submit(Job{
		EventID: "test-error-event",
		Handler: func() error {
			defer close(done)
			return expectedErr // Error should be logged, not cause crash
		},
	})

	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	select {
	case <-done:
		// Success - handler ran even though it returned error
	case <-time.After(time.Second):
		t.Fatal("Job was not executed within timeout")
	}
}

func TestWorkerPool_QueueFull(t *testing.T) {
	// 1 worker, 2 queue slots
	pool := NewWorkerPool(1, 2)
	pool.Start()
	defer pool.Shutdown(5 * time.Second)

	// Block the worker
	blocker := make(chan struct{})
	pool.Submit(Job{
		EventID: "blocking-job",
		Handler: func() error {
			<-blocker // Block until released
			return nil
		},
	})

	// Fill the queue (2 slots)
	pool.Submit(Job{EventID: "queued-1", Handler: func() error { return nil }})
	pool.Submit(Job{EventID: "queued-2", Handler: func() error { return nil }})

	// This should fail with ErrQueueFull
	err := pool.Submit(Job{EventID: "overflow", Handler: func() error { return nil }})

	if !errors.Is(err, ErrQueueFull) {
		t.Errorf("Expected ErrQueueFull, got: %v", err)
	}

	// Release the blocker to allow shutdown
	close(blocker)
}

func TestWorkerPool_PanicRecovery(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()
	defer pool.Shutdown(time.Second)

	// Track execution of jobs
	var beforePanic, afterPanic int32

	// Job that panics
	panicDone := make(chan struct{})
	pool.Submit(Job{
		EventID: "panic-job",
		Handler: func() error {
			atomic.AddInt32(&beforePanic, 1)
			close(panicDone)
			panic("intentional test panic")
		},
	})

	// Wait for panic job to execute
	select {
	case <-panicDone:
		// Panic happened
	case <-time.After(time.Second):
		t.Fatal("Panic job was not executed")
	}

	// Small delay to let panic recovery complete
	time.Sleep(50 * time.Millisecond)

	// Submit another job - worker should still be alive
	afterDone := make(chan struct{})
	err := pool.Submit(Job{
		EventID: "after-panic-job",
		Handler: func() error {
			atomic.AddInt32(&afterPanic, 1)
			close(afterDone)
			return nil
		},
	})

	if err != nil {
		t.Fatalf("Submit after panic failed: %v", err)
	}

	// Wait for second job to complete
	select {
	case <-afterDone:
		// Success - worker recovered from panic
	case <-time.After(time.Second):
		t.Fatal("Job after panic was not executed - worker may have died")
	}

	if atomic.LoadInt32(&beforePanic) != 1 {
		t.Errorf("Expected beforePanic=1, got %d", beforePanic)
	}
	if atomic.LoadInt32(&afterPanic) != 1 {
		t.Errorf("Expected afterPanic=1, got %d", afterPanic)
	}
}

func TestWorkerPool_Shutdown(t *testing.T) {
	pool := NewWorkerPool(2, 10)
	pool.Start()

	var completed int32
	jobCount := 5

	// Submit multiple jobs
	for i := 0; i < jobCount; i++ {
		pool.Submit(Job{
			EventID: "shutdown-test-job",
			Handler: func() error {
				time.Sleep(10 * time.Millisecond) // Simulate work
				atomic.AddInt32(&completed, 1)
				return nil
			},
		})
	}

	// Shutdown should wait for all jobs to complete
	err := pool.Shutdown(5 * time.Second)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// All jobs should have completed
	if atomic.LoadInt32(&completed) != int32(jobCount) {
		t.Errorf("Expected %d completed jobs, got %d", jobCount, completed)
	}
}

func TestWorkerPool_SubmitAfterShutdown(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()

	// Shutdown the pool
	err := pool.Shutdown(time.Second)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Submit after shutdown should return ErrClosed, not panic
	err = pool.Submit(Job{
		EventID: "after-shutdown",
		Handler: func() error { return nil },
	})

	if !errors.Is(err, ErrClosed) {
		t.Errorf("Expected ErrClosed after shutdown, got: %v", err)
	}
}

func TestWorkerPool_QueueLen(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	// Don't start workers - jobs will stay in queue

	// Submit without starting workers
	pool.Submit(Job{EventID: "q1", Handler: func() error { return nil }})
	pool.Submit(Job{EventID: "q2", Handler: func() error { return nil }})
	pool.Submit(Job{EventID: "q3", Handler: func() error { return nil }})

	qLen := pool.QueueLen()
	if qLen != 3 {
		t.Errorf("Expected queue length 3, got %d", qLen)
	}
}
