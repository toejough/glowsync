package filesystem

import (
	"fmt"
	"sync"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPClientPool manages a pool of SFTP clients over a single SSH connection.
// It uses a channel-based semaphore pattern for thread-safe concurrent access.
type SFTPClientPool struct {
	sshClient *ssh.Client       // underlying SSH connection
	clients   chan *sftp.Client // channel-based semaphore for pool
	maxSize   int               // maximum pool size
	mu        sync.Mutex        // protects closed flag
	closed    bool              // tracks if pool is closed
}

// NewSFTPClientPool creates a new SFTP client pool with the specified maximum size.
// The pool uses a channel-based semaphore to limit concurrent SFTP clients.
// All clients are pre-created to ensure the pool enforces maxSize correctly.
func NewSFTPClientPool(sshClient *ssh.Client, maxSize int) (*SFTPClientPool, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("pool size must be greater than 0")
	}

	pool := &SFTPClientPool{
		sshClient: sshClient,
		clients:   make(chan *sftp.Client, maxSize),
		maxSize:   maxSize,
		closed:    false,
	}

	// Pre-create ALL clients to fill the pool
	// This ensures the blocking channel pattern works correctly as a semaphore
	for i := 0; i < maxSize; i++ {
		client, err := pool.createClient()
		if err != nil {
			// Close any clients created so far
			close(pool.clients)
			for c := range pool.clients {
				_ = c.Close()
			}
			return nil, fmt.Errorf("failed to create client %d/%d: %w", i+1, maxSize, err)
		}
		pool.clients <- client
	}

	return pool, nil
}

// createClient creates a new SFTP client with concurrent writes enabled.
func (p *SFTPClientPool) createClient() (*sftp.Client, error) {
	// Enable concurrent writes for better performance (as done in Phase 1.2)
	// Note: Concurrent writes can create "holes" if writes fail mid-transfer.
	// Our error handling in CopyFileWithStats() mitigates this by deleting
	// partial files on error (see pkg/fileops/fileops.go).
	return sftp.NewClient(p.sshClient, sftp.UseConcurrentWrites(true))
}

// Acquire retrieves an SFTP client from the pool.
// Blocks until a client is available if all clients are currently in use.
// Returns an error if the pool is closed.
func (p *SFTPClientPool) Acquire() (*sftp.Client, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("pool is closed")
	}
	p.mu.Unlock()

	// Block on channel - this is the semaphore pattern
	// When pool is exhausted, goroutine blocks here until Release() returns a client
	client := <-p.clients
	return client, nil
}

// Release returns an SFTP client to the pool.
// If the pool is closed, the client is closed instead.
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

	// Return client to pool (non-blocking to avoid deadlock if pool somehow overfilled)
	select {
	case p.clients <- client:
		// Successfully returned to pool
	default:
		// This should never happen with correct implementation, but handle gracefully
		_ = client.Close()
	}
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
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// DO NOT close p.sshClient - pool doesn't own the SSH connection
	return firstErr
}
