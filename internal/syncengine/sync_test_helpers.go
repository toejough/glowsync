//go:build !release

package syncengine

import (
	"sync"
	"sync/atomic"
)

// Test helpers to access unexported fields/methods for testing

// GetDesiredWorkers returns the current desired worker count (test helper)
func (e *Engine) GetDesiredWorkers() int32 {
	return atomic.LoadInt32(&e.desiredWorkers)
}

// SetDesiredWorkers sets the desired worker count (test helper)
func (e *Engine) SetDesiredWorkers(count int) {
	atomic.StoreInt32(&e.desiredWorkers, int32(count)) //nolint:gosec // count bounded by test, no overflow risk
}

// TestWorker exposes the worker function for testing
func (e *Engine) TestWorker(wg *sync.WaitGroup, jobs <-chan *FileToSync, errors chan<- error) {
	e.worker(wg, jobs, errors)
}
