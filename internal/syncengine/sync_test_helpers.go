package syncengine

import (
	"sync/atomic"

	"github.com/joe/copy-files/pkg/filesystem"
)

// GetDesiredWorkers returns the current desired worker count (test helper).
// Only used in 6 legitimate cases where observable behavior testing isn't applicable:
// - Timing tests that detect when evaluation occurred
// - Boundary tests that verify min/max worker bounds
// - Non-deterministic tests with random perturbation
func (e *Engine) GetDesiredWorkers() int32 {
	return atomic.LoadInt32(&e.desiredWorkers)
}

// SetDesiredWorkers sets the desired worker count (test helper for initialization).
// Used to initialize test state before triggering scaling decisions.
func (e *Engine) SetDesiredWorkers(count int) {
	atomic.StoreInt32(&e.desiredWorkers, int32(count))
}

// SetSourceResizable injects a mock ResizablePool for source filesystem (test helper).
// Used by refactored tests that use observable behavior testing via ResizablePool mocks.
func (e *Engine) SetSourceResizable(pool filesystem.ResizablePool) {
	e.sourceResizable = pool
}
