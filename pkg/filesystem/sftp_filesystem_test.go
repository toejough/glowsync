//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package filesystem_test

import (
	"testing"

	"github.com/joe/copy-files/pkg/filesystem"
)

// ============================================================================
// Phase 3: SFTPFileSystem ResizablePool Interface Tests
// ============================================================================

// TestSFTPFileSystem_ImplementsResizablePool tests that SFTPFileSystem implements ResizablePool interface.
// This test will FAIL until Phase 3 implements the ResizablePool interface on SFTPFileSystem.
//
// Expected implementation:
// - SFTPFileSystem must implement all 5 methods from ResizablePool interface
// - Methods should delegate to the underlying pool
func TestSFTPFileSystem_ImplementsResizablePool(t *testing.T) {
	// Verify compile-time interface compliance:
	var _ filesystem.ResizablePool = (*filesystem.SFTPFileSystem)(nil)

	// This will fail to compile if SFTPFileSystem doesn't implement all methods
}

// TestSFTPFileSystem_ResizePool_DelegatesToPool tests that ResizePool delegates to pool's Resize.
// This test will FAIL until Phase 3 implements:
// - func (fs *SFTPFileSystem) ResizePool(targetSize int)
// - Method should call fs.pool.Resize(targetSize)
//
// Expected behavior:
// - ResizePool(8) should result in pool.TargetSize() == 8
// - Delegation should be direct (no additional logic)
func TestSFTPFileSystem_ResizePool_DelegatesToPool(t *testing.T) {
	conn, err := filesystem.Connect("localhost", 2222, "testuser")
	if err != nil {
		t.Skip("Requires SSH connection for integration test")
	}
	defer conn.Close()

	fs, err := filesystem.NewSFTPFileSystem(conn, nil)
	if err != nil {
		t.Fatalf("Creating filesystem should succeed: %v", err)
	}
	defer fs.Close()

	// Resize to 8
	fs.ResizePool(8)

	// Verify pool was resized
	if fs.PoolTargetSize() != 8 {
		t.Errorf("Pool target size should be 8 after ResizePool(8), got %d", fs.PoolTargetSize())
	}
}

// TestSFTPFileSystem_PoolSize_ReturnsActualSize tests that PoolSize returns pool's actual size.
// This test will FAIL until Phase 3 implements:
// - func (fs *SFTPFileSystem) PoolSize() int
// - Method should return fs.pool.Size()
//
// Expected behavior:
// - PoolSize() returns the current actual number of clients
// - Delegates directly to underlying pool
func TestSFTPFileSystem_PoolSize_ReturnsActualSize(t *testing.T) {
	conn, err := filesystem.Connect("localhost", 2222, "testuser")
	if err != nil {
		t.Skip("Requires SSH connection")
	}
	defer conn.Close()

	fs, err := filesystem.NewSFTPFileSystem(conn, nil)
	if err != nil {
		t.Fatalf("Creating filesystem should succeed: %v", err)
	}
	defer fs.Close()

	// Get actual size
	actualSize := fs.PoolSize()

	// Should match pool's Size()
	if actualSize <= 0 {
		t.Errorf("Pool size should be greater than 0, got %d", actualSize)
	}
	if fs.PoolSize() != actualSize {
		t.Errorf("PoolSize should return consistent value")
	}
}

// TestSFTPFileSystem_PoolTargetSize_ReturnsTargetSize tests that PoolTargetSize delegates correctly.
// This test will FAIL until Phase 3 implements:
// - func (fs *SFTPFileSystem) PoolTargetSize() int
// - Method should return fs.pool.TargetSize()
//
// Expected behavior:
// - PoolTargetSize() returns the desired pool size
// - After ResizePool(5), should return 5
func TestSFTPFileSystem_PoolTargetSize_ReturnsTargetSize(t *testing.T) {
	conn, err := filesystem.Connect("localhost", 2222, "testuser")
	if err != nil {
		t.Skip("Requires SSH connection")
	}
	defer conn.Close()

	fs, err := filesystem.NewSFTPFileSystem(conn, nil)
	if err != nil {
		t.Fatalf("Creating filesystem should succeed: %v", err)
	}
	defer fs.Close()

	// Resize pool
	fs.ResizePool(5)

	// Target size should reflect resize
	if fs.PoolTargetSize() != 5 {
		t.Errorf("PoolTargetSize should return 5 after ResizePool(5), got %d", fs.PoolTargetSize())
	}
}

// TestSFTPFileSystem_PoolMinSize_ReturnsMinSize tests that PoolMinSize delegates correctly.
// This test will FAIL until Phase 3 implements:
// - func (fs *SFTPFileSystem) PoolMinSize() int
// - Method should return fs.pool.MinSize()
//
// Expected behavior:
// - PoolMinSize() returns the minimum allowed pool size
// - Should match the min size set during pool creation
func TestSFTPFileSystem_PoolMinSize_ReturnsMinSize(t *testing.T) {
	conn, err := filesystem.Connect("localhost", 2222, "testuser")
	if err != nil {
		t.Skip("Requires SSH connection")
	}
	defer conn.Close()

	fs, err := filesystem.NewSFTPFileSystem(conn, nil)
	if err != nil {
		t.Fatalf("Creating filesystem should succeed: %v", err)
	}
	defer fs.Close()

	// Get min size
	minSize := fs.PoolMinSize()

	// Should match default min size (1)
	if minSize != 1 {
		t.Errorf("Default min size should be 1, got %d", minSize)
	}
}

// TestSFTPFileSystem_PoolMaxSize_ReturnsMaxSize tests that PoolMaxSize delegates correctly.
// This test will FAIL until Phase 3 implements:
// - func (fs *SFTPFileSystem) PoolMaxSize() int
// - Method should return fs.pool.MaxSize()
//
// Expected behavior:
// - PoolMaxSize() returns the maximum allowed pool size
// - Should match the max size set during pool creation
func TestSFTPFileSystem_PoolMaxSize_ReturnsMaxSize(t *testing.T) {
	conn, err := filesystem.Connect("localhost", 2222, "testuser")
	if err != nil {
		t.Skip("Requires SSH connection")
	}
	defer conn.Close()

	fs, err := filesystem.NewSFTPFileSystem(conn, nil)
	if err != nil {
		t.Fatalf("Creating filesystem should succeed: %v", err)
	}
	defer fs.Close()

	// Get max size
	maxSize := fs.PoolMaxSize()

	// Should match default max size (16)
	if maxSize != 16 {
		t.Errorf("Default max size should be 16, got %d", maxSize)
	}
}

// TestSFTPFileSystem_PoolConfig_DefaultValues tests pool creation with default config.
// This test will FAIL until Phase 3 implements:
// - PoolConfig struct with Initial, Min, Max fields
// - NewSFTPFileSystem updated to use defaults when config is nil
// - Defaults: Initial=4, Min=1, Max=16
//
// Expected behavior:
// - When PoolConfig is nil, use default values
// - Pool should be created with Initial=4, Min=1, Max=16
func TestSFTPFileSystem_PoolConfig_DefaultValues(t *testing.T) {
	conn, err := filesystem.Connect("localhost", 2222, "testuser")
	if err != nil {
		t.Skip("Requires SSH connection")
	}
	defer conn.Close()

	// Create filesystem without explicit config (should use defaults)
	fs, err := filesystem.NewSFTPFileSystem(conn, nil)
	if err != nil {
		t.Fatalf("Creating filesystem with default config should succeed: %v", err)
	}
	defer fs.Close()

	// Verify defaults
	if fs.PoolTargetSize() != 4 {
		t.Errorf("Default initial size should be 4, got %d", fs.PoolTargetSize())
	}
	if fs.PoolMinSize() != 1 {
		t.Errorf("Default min size should be 1, got %d", fs.PoolMinSize())
	}
	if fs.PoolMaxSize() != 16 {
		t.Errorf("Default max size should be 16, got %d", fs.PoolMaxSize())
	}
	if fs.PoolSize() != 4 {
		t.Errorf("Actual pool size should match initial size (4), got %d", fs.PoolSize())
	}
}

// TestSFTPFileSystem_PoolConfig_CustomValues tests pool creation with custom config.
// This test will FAIL until Phase 3 implements:
// - NewSFTPFileSystemWithConfig(conn, config) constructor
// - OR: NewSFTPFileSystem modified to accept optional config parameter
// - Pool creation using config.Initial, config.Min, config.Max
//
// Expected behavior:
// - When PoolConfig provided, use specified values
// - Pool should be created with custom Initial, Min, Max
func TestSFTPFileSystem_PoolConfig_CustomValues(t *testing.T) {
	conn, err := filesystem.Connect("localhost", 2222, "testuser")
	if err != nil {
		t.Skip("Requires SSH connection")
	}
	defer conn.Close()

	// Create custom config
	config := &filesystem.PoolConfig{
		InitialSize: 6,
		MinSize:     2,
		MaxSize:     12,
	}

	// Create filesystem with custom config
	fs, err := filesystem.NewSFTPFileSystem(conn, config)
	if err != nil {
		t.Fatalf("Creating filesystem with custom config should succeed: %v", err)
	}
	defer fs.Close()

	// Verify custom values
	if fs.PoolTargetSize() != 6 {
		t.Errorf("Initial size should be 6, got %d", fs.PoolTargetSize())
	}
	if fs.PoolMinSize() != 2 {
		t.Errorf("Min size should be 2, got %d", fs.PoolMinSize())
	}
	if fs.PoolMaxSize() != 12 {
		t.Errorf("Max size should be 12, got %d", fs.PoolMaxSize())
	}
	if fs.PoolSize() != 6 {
		t.Errorf("Actual pool size should match initial size (6), got %d", fs.PoolSize())
	}
}

// TestSFTPFileSystem_PoolConfig_InvalidValues_ReturnsError tests validation of pool config.
// This test will FAIL until Phase 3 implements validation:
// - Initial must be >= Min
// - Initial must be <= Max
// - Min must be > 0
// - Max must be >= Min
//
// Expected behavior:
// - Invalid config should return error
// - Error message should indicate which constraint failed
func TestSFTPFileSystem_PoolConfig_InvalidValues_ReturnsError(t *testing.T) {
	conn, err := filesystem.Connect("localhost", 2222, "testuser")
	if err != nil {
		t.Skip("Requires SSH connection")
	}
	defer conn.Close()

	// Test cases for invalid config
	testCases := []struct {
		name   string
		config *filesystem.PoolConfig
		errMsg string
	}{
		{
			name:   "Initial > Max",
			config: &filesystem.PoolConfig{InitialSize: 10, MinSize: 1, MaxSize: 5},
			errMsg: "initialSize (10) must be <= maxSize (5)",
		},
		{
			name:   "Initial < Min",
			config: &filesystem.PoolConfig{InitialSize: 2, MinSize: 5, MaxSize: 10},
			errMsg: "initialSize (2) must be >= minSize (5)",
		},
		{
			name:   "Min = 0",
			config: &filesystem.PoolConfig{InitialSize: 4, MinSize: 0, MaxSize: 10},
			errMsg: "minSize must be greater than 0",
		},
		{
			name:   "Min < 0",
			config: &filesystem.PoolConfig{InitialSize: 4, MinSize: -1, MaxSize: 10},
			errMsg: "minSize must be greater than 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs, err := filesystem.NewSFTPFileSystem(conn, tc.config)
			if err == nil {
				t.Errorf("Should return error for invalid config")
				if fs != nil {
					fs.Close()
				}
			}
			if fs != nil {
				t.Errorf("Filesystem should be nil on error")
			}
		})
	}
}

// TestSFTPFileSystem_ResizePool_IntegrationWithSyncEngine tests that pool resize works end-to-end.
// This test will FAIL until Phase 3 is complete.
//
// Expected behavior:
// - Filesystem implements ResizablePool interface
// - ResizePool(N) adjusts pool size
// - File operations work correctly at different pool sizes
// - No deadlocks or race conditions during resize
func TestSFTPFileSystem_ResizePool_IntegrationWithSyncEngine(t *testing.T) {
	conn, err := filesystem.Connect("localhost", 2222, "testuser")
	if err != nil {
		t.Skip("Requires SSH connection")
	}
	defer conn.Close()

	fs, err := filesystem.NewSFTPFileSystem(conn, nil)
	if err != nil {
		t.Fatalf("Creating filesystem should succeed: %v", err)
	}
	defer fs.Close()

	// Initial pool size (default 4)
	if fs.PoolTargetSize() != 4 {
		t.Errorf("Default target size should be 4, got %d", fs.PoolTargetSize())
	}

	// Resize up to 8
	fs.ResizePool(8)
	if fs.PoolTargetSize() != 8 {
		t.Errorf("Target size should be 8 after resize, got %d", fs.PoolTargetSize())
	}

	// Resize down to 2
	fs.ResizePool(2)
	if fs.PoolTargetSize() != 2 {
		t.Errorf("Target size should be 2 after resize, got %d", fs.PoolTargetSize())
	}
}

// TestSFTPFileSystem_TypeAssertion_ToResizablePool tests runtime type assertion.
// This test will FAIL until Phase 3 implements ResizablePool interface.
//
// Expected behavior:
// - Type assertion from FileSystem to ResizablePool should succeed
// - Can call ResizablePool methods through interface
func TestSFTPFileSystem_TypeAssertion_ToResizablePool(t *testing.T) {
	conn, err := filesystem.Connect("localhost", 2222, "testuser")
	if err != nil {
		t.Skip("Requires SSH connection")
	}
	defer conn.Close()

	fs, err := filesystem.NewSFTPFileSystem(conn, nil)
	if err != nil {
		t.Fatalf("Creating filesystem should succeed: %v", err)
	}
	defer fs.Close()

	// Type assert to ResizablePool
	resizable, ok := interface{}(fs).(filesystem.ResizablePool)
	if !ok {
		t.Errorf("SFTPFileSystem should implement ResizablePool interface")
	}
	if resizable == nil {
		t.Errorf("Type assertion should return non-nil ResizablePool")
	}

	// Verify can call methods through interface
	resizable.ResizePool(5)
	if resizable.PoolTargetSize() != 5 {
		t.Errorf("PoolTargetSize should return 5 after ResizePool(5), got %d", resizable.PoolTargetSize())
	}
}

// TestSFTPFileSystem_API_Exists tests that ResizablePool methods exist.
// This is a compile-time check for Phase 3 implementation.
func TestSFTPFileSystem_API_Exists(t *testing.T) {
	t.Parallel()

	// Verify the API exists by checking function signatures compile
	_ = (*filesystem.SFTPFileSystem).Close
	_ = (*filesystem.SFTPFileSystem).Scan
	_ = (*filesystem.SFTPFileSystem).Open
	_ = (*filesystem.SFTPFileSystem).Create

	// Phase 3: ResizablePool methods
	_ = (*filesystem.SFTPFileSystem).ResizePool
	_ = (*filesystem.SFTPFileSystem).PoolSize
	_ = (*filesystem.SFTPFileSystem).PoolTargetSize
	_ = (*filesystem.SFTPFileSystem).PoolMinSize
	_ = (*filesystem.SFTPFileSystem).PoolMaxSize

	// Test passes if code compiles
}
