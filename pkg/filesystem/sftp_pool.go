package filesystem

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPClientPool manages a pool of SFTP clients over a single SSH connection.
// It uses a channel-based semaphore pattern for thread-safe concurrent access.
type SFTPClientPool struct {
	sshClient  *ssh.Client       // underlying SSH connection
	clients    chan *sftp.Client // channel-based semaphore for pool
	maxSize    int               // maximum allowed pool size
	minSize    int               // minimum pool size (never scale below)
	targetSize int32             // desired pool size (atomic)
	actualSize int32             // current actual number of clients (atomic)
	mu         sync.Mutex        // protects closed flag
	closed     bool              // tracks if pool is closed
}

// NewSFTPClientPoolWithLimits creates a new SFTP client pool with size limits.
// The pool supports adaptive sizing between minSize and maxSize.
// initialSize clients are pre-created to fill the pool.
// Parameters must satisfy: 0 < minSize <= initialSize <= maxSize
func NewSFTPClientPoolWithLimits(sshClient *ssh.Client, initialSize, minSize, maxSize int) (*SFTPClientPool, error) {
	// Validate bounds
	if minSize <= 0 {
		return nil, fmt.Errorf("minSize must be greater than 0, got %d", minSize) //nolint:err113,lll // Validation error with actual values
	}
	if initialSize < minSize {
		return nil, fmt.Errorf("initialSize (%d) must be >= minSize (%d)", initialSize, minSize) //nolint:err113,lll // Validation error with actual values
	}
	if initialSize > maxSize {
		return nil, fmt.Errorf("initialSize (%d) must be <= maxSize (%d)", initialSize, maxSize) //nolint:err113,lll // Validation error with actual values
	}

	pool := &SFTPClientPool{
		sshClient:  sshClient,
		clients:    make(chan *sftp.Client, maxSize),
		minSize:    minSize,
		maxSize:    maxSize,
		targetSize: int32(initialSize), //nolint:gosec,lll // initialSize validated to be small (≤maxSize, typically ≤16), no overflow risk
		actualSize: int32(initialSize), //nolint:gosec,lll // initialSize validated to be small (≤maxSize, typically ≤16), no overflow risk
		closed:     false,
	}

	// Pre-create initialSize clients to fill the pool
	for i := range initialSize { //nolint:varnamelen // i is idiomatic loop counter
		client, err := pool.createClient()
		if err != nil {
			// Close any clients created so far
			close(pool.clients)
			for c := range pool.clients {
				_ = c.Close()
			}

			return nil, fmt.Errorf("failed to create client %d/%d: %w", i+1, initialSize, err)
		}
		pool.clients <- client
	}

	return pool, nil
}

// Acquire retrieves an SFTP client from the pool.
// Blocks until a client is available if all clients are currently in use.
// Returns an error if the pool is closed.
func (p *SFTPClientPool) Acquire() (*sftp.Client, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("pool is closed") //nolint:err113,perfsprint // Simple state error
	}
	p.mu.Unlock()

	// Block on channel - this is the semaphore pattern
	// When pool is exhausted, goroutine blocks here until Release() returns a client
	client := <-p.clients

	return client, nil
}

// Close closes the pool and all SFTP clients in it.
// After Close(), Acquire() will return an error and Release() will close clients.
// Close is idempotent - safe to call multiple times.
// Note: Does NOT close the underlying SSH connection (pool doesn't own it).
func (p *SFTPClientPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil // Already closed, safe to call multiple times
	}
	p.closed = true
	p.mu.Unlock()

	// Close and drain the channel
	close(p.clients)

	var firstErr error
	for client := range p.clients {
		if err := client.Close(); err != nil && firstErr == nil { //nolint:noinlineerr,lll // Inline error check is idiomatic for cleanup operations
			firstErr = err
		}
	}

	// Reset size counters
	atomic.StoreInt32(&p.actualSize, 0)
	atomic.StoreInt32(&p.targetSize, 0)

	// DO NOT close p.sshClient - pool doesn't own the SSH connection
	return firstErr
}

// MaxSize returns the maximum pool size.
func (p *SFTPClientPool) MaxSize() int {
	return p.maxSize
}

// MinSize returns the minimum pool size.
func (p *SFTPClientPool) MinSize() int {
	return p.minSize
}

// Release returns an SFTP client to the pool.
// If the pool is closed, the client is closed instead.
// Implements lazy scale-down: if actualSize > targetSize, closes the client.
// Uses CAS to prevent race conditions during concurrent scale-down.
// Handles nil clients gracefully by returning immediately.
func (p *SFTPClientPool) Release(client *sftp.Client) {
	if client == nil {
		return
	}

	p.mu.Lock()
	poolClosed := p.closed
	p.mu.Unlock()

	if poolClosed {
		// Pool is closed, close the client instead of returning it
		_ = client.Close()
		return
	}

	// Check if we should scale down (lazy)
	for {
		target := atomic.LoadInt32(&p.targetSize)
		actual := atomic.LoadInt32(&p.actualSize)

		if actual <= target {
			break // At or below target, return to pool
		}

		// Try to scale down using CAS
		if atomic.CompareAndSwapInt32(&p.actualSize, actual, actual-1) {
			// We won the race - close this client
			_ = client.Close()
			return
		}
		// CAS failed, retry
	}

	// Return to pool
	select {
	case p.clients <- client:
		// Success
	default:
		// Pool full (shouldn't happen)
		atomic.AddInt32(&p.actualSize, -1)
		_ = client.Close()
	}
}

// Resize changes the target pool size, clamped to [minSize, maxSize].
// Scale-up is eager (creates clients immediately).
// Scale-down is lazy (closes clients on Release).
func (p *SFTPClientPool) Resize(targetSize int) {
	// Clamp to [minSize, maxSize]
	clamped := min(max(targetSize, p.minSize), p.maxSize)

	atomic.StoreInt32(&p.targetSize, int32(clamped)) //nolint:gosec,lll // clamped to [minSize, maxSize], typically small values (≤16), no overflow risk

	// Eager scale-up: try to add clients now
	p.scaleUp()
}

// Size returns the current actual number of clients in the pool.
func (p *SFTPClientPool) Size() int {
	return int(atomic.LoadInt32(&p.actualSize))
}

// TargetSize returns the desired pool size.
func (p *SFTPClientPool) TargetSize() int {
	return int(atomic.LoadInt32(&p.targetSize))
}

// createClient creates a new SFTP client with concurrent writes enabled.
func (p *SFTPClientPool) createClient() (*sftp.Client, error) {
	// Enable concurrent writes for better performance (as done in Phase 1.2)
	// Note: Concurrent writes can create "holes" if writes fail mid-transfer.
	// Our error handling in CopyFileWithStats() mitigates this by deleting
	// partial files on error (see pkg/fileops/fileops.go).
	return sftp.NewClient(p.sshClient, sftp.UseConcurrentWrites(true)) //nolint:wrapcheck,lll // External package error, wrapped by caller in NewClient()
}

// scaleUp attempts to create new clients to reach targetSize.
// Called during eager scale-up in Resize().
// Handles partial scale-up gracefully if client creation fails.
func (p *SFTPClientPool) scaleUp() {
	for {
		target := atomic.LoadInt32(&p.targetSize)
		actual := atomic.LoadInt32(&p.actualSize)

		if actual >= target {
			break // At or above target
		}

		// Try to create a new client
		client, err := p.createClient()
		if err != nil {
			break // Can't scale up further, partial scale-up is ok
		}

		// Non-blocking channel send
		select {
		case p.clients <- client:
			atomic.AddInt32(&p.actualSize, 1)
		default:
			// Channel full, someone else scaled up
			_ = client.Close()
			return
		}
	}
}
