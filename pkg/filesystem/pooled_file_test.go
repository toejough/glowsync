//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package filesystem_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joe/copy-files/pkg/filesystem"
	"github.com/pkg/sftp"
)

// TestPooledSFTPFile_Read_DelegatesToWrappedFile tests that Read delegates to underlying file.
func TestPooledSFTPFile_Read_DelegatesToWrappedFile(t *testing.T) {
	mockFile := &mockSFTPFile{
		readFunc: func(p []byte) (int, error) {
			copy(p, []byte("test data"))
			return 9, nil
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	buf := make([]byte, 100)
	n, err := pooledFile.Read(buf)

	if err != nil {
		t.Errorf("Read should succeed, got error: %v", err)
	}
	if n != 9 {
		t.Errorf("Should read 9 bytes, got %d", n)
	}
	if string(buf[:n]) != "test data" {
		t.Errorf("Expected 'test data', got %q", string(buf[:n]))
	}
}

// TestPooledSFTPFile_Write_DelegatesToWrappedFile tests that Write delegates to underlying file.
func TestPooledSFTPFile_Write_DelegatesToWrappedFile(t *testing.T) {
	writtenData := []byte{}
	mockFile := &mockSFTPFile{
		writeFunc: func(p []byte) (int, error) {
			writtenData = append(writtenData, p...)
			return len(p), nil
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	data := []byte("test write")
	n, err := pooledFile.Write(data)

	if err != nil {
		t.Errorf("Write should succeed, got error: %v", err)
	}
	if n != 10 {
		t.Errorf("Should write 10 bytes, got %d", n)
	}
	if string(writtenData) != string(data) {
		t.Errorf("Expected %q, got %q", data, writtenData)
	}
}

// TestPooledSFTPFile_Stat_DelegatesToWrappedFile tests that Stat delegates to underlying file.
func TestPooledSFTPFile_Stat_DelegatesToWrappedFile(t *testing.T) {
	mockFileInfo := &mockFileInfo{
		name:  "test.txt",
		size:  1024,
		mode:  0644,
		mtime: time.Now(),
	}
	mockFile := &mockSFTPFile{
		statFunc: func() (os.FileInfo, error) {
			return mockFileInfo, nil
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	info, err := pooledFile.Stat()

	if err != nil {
		t.Errorf("Stat should succeed, got error: %v", err)
	}
	if info.Name() != "test.txt" {
		t.Errorf("Expected name 'test.txt', got %q", info.Name())
	}
	if info.Size() != 1024 {
		t.Errorf("Expected size 1024, got %d", info.Size())
	}
}

// TestPooledSFTPFile_Close_ClosesFileAndReleasesClient tests the core auto-release behavior.
func TestPooledSFTPFile_Close_ClosesFileAndReleasesClient(t *testing.T) {
	fileClosed := false
	clientReleased := false

	mockFile := &mockSFTPFile{
		closeFunc: func() error {
			fileClosed = true
			return nil
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{
		releaseFunc: func(client *sftp.Client) {
			clientReleased = true
		},
	}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	err = pooledFile.Close()

	if err != nil {
		t.Errorf("Close should succeed, got error: %v", err)
	}
	if !fileClosed {
		t.Error("File should be closed")
	}
	if !clientReleased {
		t.Error("Client should be released")
	}
}

// TestPooledSFTPFile_Close_ReturnsFileCloseError tests error propagation from file.Close().
func TestPooledSFTPFile_Close_ReturnsFileCloseError(t *testing.T) {
	closeErr := errors.New("file close failed")
	clientReleased := false

	mockFile := &mockSFTPFile{
		closeFunc: func() error {
			return closeErr
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{
		releaseFunc: func(client *sftp.Client) {
			clientReleased = true
		},
	}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	err = pooledFile.Close()

	if err != closeErr {
		t.Errorf("Expected error %v, got %v", closeErr, err)
	}
	if !clientReleased {
		t.Error("Client should be released even when file.Close() fails")
	}
}

// TestPooledSFTPFile_DoubleClose_IsSafe tests that Close is idempotent.
func TestPooledSFTPFile_DoubleClose_IsSafe(t *testing.T) {
	closeCount := 0
	releaseCount := 0

	mockFile := &mockSFTPFile{
		closeFunc: func() error {
			closeCount++
			if closeCount > 1 {
				return errors.New("file already closed")
			}
			return nil
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{
		releaseFunc: func(client *sftp.Client) {
			releaseCount++
		},
	}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	// First close
	err1 := pooledFile.Close()
	if err1 != nil {
		t.Errorf("First close failed: %v", err1)
	}

	// Second close - should be no-op
	err2 := pooledFile.Close()
	if err2 != nil {
		t.Errorf("Second close should not error, got: %v", err2)
	}

	if closeCount != 1 {
		t.Errorf("File should only be closed once, got %d", closeCount)
	}
	if releaseCount != 1 {
		t.Errorf("Client should only be released once, got %d", releaseCount)
	}
}

// TestPooledSFTPFile_Close_ReleasesClientEvenWhenFileCloseErrors tests guaranteed release.
func TestPooledSFTPFile_Close_ReleasesClientEvenWhenFileCloseErrors(t *testing.T) {
	closeErr := errors.New("close failed")
	clientReleased := false

	mockFile := &mockSFTPFile{
		closeFunc: func() error {
			return closeErr
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{
		releaseFunc: func(client *sftp.Client) {
			clientReleased = true
		},
	}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	err = pooledFile.Close()

	if err != closeErr {
		t.Errorf("Expected error %v, got %v", closeErr, err)
	}
	if !clientReleased {
		t.Error("Client must be released even if file.Close() fails")
	}
}

// TestPooledSFTPFile_Close_WhenPoolClosed_StillClosesFile tests behavior with closed pool.
func TestPooledSFTPFile_Close_WhenPoolClosed_StillClosesFile(t *testing.T) {
	fileClosed := false

	mockFile := &mockSFTPFile{
		closeFunc: func() error {
			fileClosed = true
			return nil
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{
		releaseFunc: func(client *sftp.Client) {
			// Pool is closed, but release should still be called
		},
	}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	// Close pooled file
	err = pooledFile.Close()
	if err != nil {
		t.Errorf("Close should succeed, got error: %v", err)
	}

	if !fileClosed {
		t.Error("File should be closed even if pool is closed")
	}
}

// TestPooledSFTPFile_ReadWrite_AfterClose_ReturnsError tests usage after close.
func TestPooledSFTPFile_ReadWrite_AfterClose_ReturnsError(t *testing.T) {
	mockFile := &mockSFTPFile{
		closeFunc: func() error { return nil },
		readFunc: func(p []byte) (int, error) {
			return 0, errors.New("should not be called")
		},
		writeFunc: func(p []byte) (int, error) {
			return 0, errors.New("should not be called")
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	pooledFile.Close()

	_, readErr := pooledFile.Read(make([]byte, 10))
	if !errors.Is(readErr, fs.ErrClosed) {
		t.Errorf("Read after Close should return fs.ErrClosed, got: %v", readErr)
	}

	_, writeErr := pooledFile.Write([]byte("data"))
	if !errors.Is(writeErr, fs.ErrClosed) {
		t.Errorf("Write after Close should return fs.ErrClosed, got: %v", writeErr)
	}
}

// TestPooledSFTPFile_ImplementsFileInterface tests interface compliance.
func TestPooledSFTPFile_ImplementsFileInterface(t *testing.T) {
	// This should compile if PooledSFTPFile implements filesystem.File
	var _ filesystem.File = (*filesystem.PooledSFTPFile)(nil)
}

// TestPooledSFTPFile_NilSafety_NilFile tests nil file handling.
func TestPooledSFTPFile_NilSafety_NilFile(t *testing.T) {
	mockClient := &sftp.Client{}
	mockPool := &mockPool{}

	pooledFile, err := filesystem.NewPooledSFTPFile(nil, mockClient, mockPool)
	if err == nil {
		t.Error("Should error on nil file")
	}
	if pooledFile != nil {
		t.Error("Should return nil pooledFile on error")
	}
}

// TestPooledSFTPFile_NilSafety_NilClient tests nil client handling.
func TestPooledSFTPFile_NilSafety_NilClient(t *testing.T) {
	mockFile := &mockSFTPFile{}
	mockPool := &mockPool{}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, nil, mockPool)
	if err == nil {
		t.Error("Should error on nil client")
	}
	if pooledFile != nil {
		t.Error("Should return nil pooledFile on error")
	}
}

// TestPooledSFTPFile_NilSafety_NilPool tests nil pool handling.
func TestPooledSFTPFile_NilSafety_NilPool(t *testing.T) {
	mockFile := &mockSFTPFile{}
	mockClient := &sftp.Client{}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, nil)
	if err == nil {
		t.Error("Should error on nil pool")
	}
	if pooledFile != nil {
		t.Error("Should return nil pooledFile on error")
	}
}

// TestPooledSFTPFile_ConcurrentClose_IsSafe tests concurrent close operations.
func TestPooledSFTPFile_ConcurrentClose_IsSafe(t *testing.T) {
	closeCount := atomic.Int32{}
	releaseCount := atomic.Int32{}

	mockFile := &mockSFTPFile{
		closeFunc: func() error {
			closeCount.Add(1)
			time.Sleep(10 * time.Millisecond) // Simulate slow close
			return nil
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{
		releaseFunc: func(client *sftp.Client) {
			releaseCount.Add(1)
		},
	}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pooledFile.Close()
		}()
	}
	wg.Wait()

	if closeCount.Load() != 1 {
		t.Errorf("File should close exactly once, got %d", closeCount.Load())
	}
	if releaseCount.Load() != 1 {
		t.Errorf("Client should release exactly once, got %d", releaseCount.Load())
	}
}

// TestPooledSFTPFile_DeferPattern_WorksCorrectly tests typical usage pattern.
func TestPooledSFTPFile_DeferPattern_WorksCorrectly(t *testing.T) {
	fileClosed := false
	clientReleased := false

	mockFile := &mockSFTPFile{
		writeFunc: func(p []byte) (int, error) {
			return len(p), nil
		},
		closeFunc: func() error {
			fileClosed = true
			return nil
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{
		releaseFunc: func(client *sftp.Client) {
			clientReleased = true
		},
	}

	func() {
		pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
		if err != nil {
			t.Fatalf("NewPooledSFTPFile failed: %v", err)
		}
		defer pooledFile.Close() // Client auto-released here

		// Use file
		_, err = pooledFile.Write([]byte("test data"))
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
	}()

	// After defer fires, client should be released
	if !fileClosed {
		t.Error("File should be closed after defer")
	}
	if !clientReleased {
		t.Error("Client should be released after defer")
	}
}

// TestPooledSFTPFile_ErrorDuringRead_ClientStillReleasedOnClose tests error handling.
func TestPooledSFTPFile_ErrorDuringRead_ClientStillReleasedOnClose(t *testing.T) {
	readErr := errors.New("read failed")
	clientReleased := false

	mockFile := &mockSFTPFile{
		readFunc: func(p []byte) (int, error) {
			return 0, readErr
		},
		closeFunc: func() error {
			return nil
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{
		releaseFunc: func(client *sftp.Client) {
			clientReleased = true
		},
	}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	// Read fails
	_, err = pooledFile.Read(make([]byte, 10))
	if err != readErr {
		t.Errorf("Expected error %v, got %v", readErr, err)
	}

	// Close should still release client
	err = pooledFile.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if !clientReleased {
		t.Error("Client must be released even after read error")
	}
}

// TestPooledSFTPFile_MultipleReadsWrites_WorkCorrectly tests normal operations.
func TestPooledSFTPFile_MultipleReadsWrites_WorkCorrectly(t *testing.T) {
	buffer := []byte{}
	mockFile := &mockSFTPFile{
		writeFunc: func(p []byte) (int, error) {
			buffer = append(buffer, p...)
			return len(p), nil
		},
		readFunc: func(p []byte) (int, error) {
			n := copy(p, buffer)
			return n, nil
		},
		closeFunc: func() error {
			return nil
		},
	}
	mockClient := &sftp.Client{}
	mockPool := &mockPool{}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile, mockClient, mockPool)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}
	defer pooledFile.Close()

	// Multiple writes
	_, err = pooledFile.Write([]byte("hello "))
	if err != nil {
		t.Errorf("First write failed: %v", err)
	}
	_, err = pooledFile.Write([]byte("world"))
	if err != nil {
		t.Errorf("Second write failed: %v", err)
	}

	// Multiple reads
	buf := make([]byte, 5)
	n, err := pooledFile.Read(buf)
	if err != nil {
		t.Errorf("First read failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to read 5 bytes, got %d", n)
	}
	if string(buf) != "hello" {
		t.Errorf("Expected 'hello', got %q", string(buf))
	}
}

// TestPooledSFTPFile_Documentation_UsageExample documents expected usage.
func TestPooledSFTPFile_Documentation_UsageExample(t *testing.T) {
	// Expected usage in SFTPFileSystem.Create():
	//
	// func (fs *SFTPFileSystem) Create(path string) (filesystem.File, error) {
	//     client, err := fs.pool.Acquire()
	//     if err != nil {
	//         return nil, err
	//     }
	//
	//     file, err := client.Create(path)
	//     if err != nil {
	//         fs.pool.Release(client) // Release on error
	//         return nil, err
	//     }
	//
	//     // Wrap in pooled file - client will auto-release on Close()
	//     return filesystem.NewPooledSFTPFile(file, client, fs.pool), nil
	// }
	//
	// User code (unchanged from before):
	// file, err := fs.Create("/remote/path/file.txt")
	// if err != nil {
	//     return err
	// }
	// defer file.Close() // This now auto-releases the pool client!
	//
	// _, err = file.Write(data)
	// // ...
}

// Mock types for testing (will be implemented in test helpers)

// mockSFTPFile is a mock implementation of *sftp.File for testing.
// Phase 2.3 tests will use this to avoid real SSH connections.
type mockSFTPFile struct {
	readFunc  func([]byte) (int, error)
	writeFunc func([]byte) (int, error)
	closeFunc func() error
	statFunc  func() (os.FileInfo, error)
}

func (m *mockSFTPFile) Read(p []byte) (int, error) {
	if m.readFunc != nil {
		return m.readFunc(p)
	}
	return 0, io.EOF
}

func (m *mockSFTPFile) Write(p []byte) (int, error) {
	if m.writeFunc != nil {
		return m.writeFunc(p)
	}
	return len(p), nil
}

func (m *mockSFTPFile) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockSFTPFile) Stat() (os.FileInfo, error) {
	if m.statFunc != nil {
		return m.statFunc()
	}
	return &mockFileInfo{}, nil
}

// mockFileInfo is a mock implementation of os.FileInfo for testing.
type mockFileInfo struct {
	name  string
	size  int64
	mode  os.FileMode
	mtime time.Time
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.mtime }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// mockPool is a mock implementation of SFTPClientPool for testing.
type mockPool struct {
	acquireFunc func() (*sftp.Client, error)
	releaseFunc func(*sftp.Client)
	closeFunc   func() error
}

func (m *mockPool) Acquire() (*sftp.Client, error) {
	if m.acquireFunc != nil {
		return m.acquireFunc()
	}
	return nil, errors.New("not implemented")
}

func (m *mockPool) Release(client *sftp.Client) {
	if m.releaseFunc != nil {
		m.releaseFunc(client)
	}
}

func (m *mockPool) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}
