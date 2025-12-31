package filesystem

// ResizablePool is an optional interface for filesystems with resizable connection pools.
// Filesystems that support dynamic pool sizing (e.g., SFTP) should implement this interface.
// The sync engine can detect and use this interface to match pool size with worker count.
type ResizablePool interface {
	// ResizePool sets the target pool size.
	// The pool will adjust toward this target (scale-up eager, scale-down lazy).
	// Size is clamped to [PoolMinSize, PoolMaxSize].
	ResizePool(targetSize int)

	// PoolSize returns the current actual number of connections in the pool.
	PoolSize() int

	// PoolTargetSize returns the current target pool size.
	PoolTargetSize() int

	// PoolMinSize returns the minimum allowed pool size.
	PoolMinSize() int

	// PoolMaxSize returns the maximum allowed pool size.
	PoolMaxSize() int
}
