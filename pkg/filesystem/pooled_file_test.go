//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package filesystem_test

//go:generate impgen --dependency filesystem.SftpFile
//go:generate impgen --dependency filesystem.ClientPool
//go:generate impgen --dependency fs.FileInfo

import (
	"errors"
	"io"
	"io/fs"
	"sync"
	"testing"
	"time"

	"github.com/joe/copy-files/pkg/filesystem"
	"github.com/pkg/sftp"
	"github.com/toejough/imptest/imptest"
)

// TestImptestV2APIValidation validates understanding of imptest V2 API before migrating all tests.
//
//nolint:funlen // Comprehensive imptest V2 API validation with multiple mock scenarios
func TestImptestV2APIValidation(t *testing.T) {
	t.Parallel()

	// Pattern discovered: Expectations must be set up in goroutines that run concurrently
	// with the code under test, because imptest uses channels internally.

	t.Run("Basic Read with return value injection", func(t *testing.T) {
		t.Parallel()

		mockFile := MockSftpFile(t)
		reader := mockFile.Mock.(io.Reader) //nolint:forcetypeassert // Test code - mock always implements io.Reader

		// Set up expectation in goroutine
		done := make(chan struct{})
		go func() {
			defer close(done)
			// For Read, we accept any buffer since we don't control it
			readCall := mockFile.Method.Read.ExpectCalledWithMatches(imptest.Any())
			readCall.InjectReturnValues(9, nil)
		}()

		// Call the code
		buf := make([]byte, 100)
		n, err := reader.Read(buf)
		<-done

		// Verify
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
		if n != 9 {
			t.Errorf("Expected 9 bytes, got %d", n)
		}
	})

	t.Run("Write with GetArgs to verify side effects", func(t *testing.T) {
		t.Parallel()

		mockFile := MockSftpFile(t)
		writer := mockFile.Mock.(io.Writer) //nolint:forcetypeassert // Test code - mock always implements io.Writer

		testData := []byte("test data")
		var writeCall *SftpFileMockWriteCall

		// Set up expectation in goroutine
		done := make(chan struct{})
		go func() {
			defer close(done)
			// Accept any byte slice - we'll verify using GetArgs
			writeCall = mockFile.Method.Write.ExpectCalledWithMatches(imptest.Any())
			writeCall.InjectReturnValues(len(testData), nil)
		}()

		// Call the code
		n, err := writer.Write(testData)
		<-done

		// Verify return values
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
		if n != len(testData) {
			t.Errorf("Expected %d bytes, got %d", len(testData), n)
		}

		// Verify side effect using GetArgs
		args := writeCall.GetArgs()
		if string(args.P) != string(testData) {
			t.Errorf("Expected data %q, got %q", testData, args.P)
		}
	})

	t.Run("Error injection", func(t *testing.T) {
		t.Parallel()

		mockFile := MockSftpFile(t)
		closer := mockFile.Mock.(io.Closer) //nolint:forcetypeassert // Test code - mock always implements io.Closer

		expectedErr := errors.New("close failed")

		// Set up expectation in goroutine
		done := make(chan struct{})
		go func() {
			defer close(done)
			closeCall := mockFile.Method.Close.ExpectCalledWithExactly()
			closeCall.InjectReturnValues(expectedErr)
		}()

		// Call the code
		err := closer.Close()
		<-done

		// Verify error propagation
		if err != expectedErr { //nolint:errorlint // Test code - verifying exact error object, not wrapped error
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})

	// Note: ClientPool mock is now exported and works the same way as above
	// The pattern is the same: create mock, set expectations in goroutine, call methods.
}

// TestPooledSFTPFile_Close_ClosesFileAndReleasesClient tests the core auto-release behavior.
func TestPooledSFTPFile_Close_ClosesFileAndReleasesClient(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Expect file.Close() to be called
		closeCall := mockFile.Method.Close.ExpectCalledWithExactly()
		closeCall.InjectReturnValues(nil)

		// Expect pool.Release() to be called
		releaseCall := mockPool.Method.Release.Eventually.ExpectCalledWithMatches(imptest.Any())
		releaseCall.InjectReturnValues()
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	err = pooledFile.Close()
	<-done

	if err != nil {
		t.Errorf("Close should succeed, got error: %v", err)
	}
	// Success means both Close() and Release() were called as expected
}

// TestPooledSFTPFile_Close_ReleasesClientEvenWhenFileCloseErrors tests guaranteed release.
func TestPooledSFTPFile_Close_ReleasesClientEvenWhenFileCloseErrors(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("close failed")
	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Expect file.Close() to return error
		closeCall := mockFile.Method.Close.ExpectCalledWithExactly()
		closeCall.InjectReturnValues(closeErr)

		// Expect pool.Release() still called despite error
		releaseCall := mockPool.Method.Release.Eventually.ExpectCalledWithMatches(imptest.Any())
		releaseCall.InjectReturnValues()
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	err = pooledFile.Close()
	<-done

	if err != closeErr { //nolint:errorlint // Test code - verifying exact error object, not wrapped error
		t.Errorf("Expected error %v, got %v", closeErr, err)
	}
	// Success means Release() was called even though Close() errored
}

// TestPooledSFTPFile_Close_ReturnsFileCloseError tests error propagation from file.Close().
func TestPooledSFTPFile_Close_ReturnsFileCloseError(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("file close failed")
	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Expect file.Close() to return error
		closeCall := mockFile.Method.Close.ExpectCalledWithExactly()
		closeCall.InjectReturnValues(closeErr)

		// Expect pool.Release() still called despite error
		releaseCall := mockPool.Method.Release.Eventually.ExpectCalledWithMatches(imptest.Any())
		releaseCall.InjectReturnValues()
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	err = pooledFile.Close()
	<-done

	if err != closeErr { //nolint:errorlint // Test code - verifying exact error object, not wrapped error
		t.Errorf("Expected error %v, got %v", closeErr, err)
	}
	// Success means Release() was called even though Close() errored
}

// TestPooledSFTPFile_Close_WhenPoolClosed_StillClosesFile tests behavior with closed pool.
func TestPooledSFTPFile_Close_WhenPoolClosed_StillClosesFile(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Expect file.Close() to be called
		closeCall := mockFile.Method.Close.ExpectCalledWithExactly()
		closeCall.InjectReturnValues(nil)

		// Expect pool.Release() to be called (even if pool is closed, it's still called)
		releaseCall := mockPool.Method.Release.Eventually.ExpectCalledWithMatches(imptest.Any())
		releaseCall.InjectReturnValues()
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	// Close pooled file
	err = pooledFile.Close()
	<-done

	if err != nil {
		t.Errorf("Close should succeed, got error: %v", err)
	}
	// Success means both Close() and Release() were called
}

// TestPooledSFTPFile_ConcurrentClose_IsSafe tests concurrent close operations.
func TestPooledSFTPFile_ConcurrentClose_IsSafe(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Expect exactly one Close() even though 10 goroutines call pooledFile.Close()
		closeCall := mockFile.Method.Close.ExpectCalledWithExactly()
		time.Sleep(10 * time.Millisecond) // Simulate slow close
		closeCall.InjectReturnValues(nil)

		// Expect exactly one Release()
		releaseCall := mockPool.Method.Release.Eventually.ExpectCalledWithMatches(imptest.Any())
		releaseCall.InjectReturnValues()
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_ = pooledFile.Close()
		})
	}
	wg.Wait()
	<-done

	// Success means only one Close() and one Release() were called despite 10 concurrent calls
}

// TestPooledSFTPFile_DeferPattern_WorksCorrectly tests typical usage pattern.
func TestPooledSFTPFile_DeferPattern_WorksCorrectly(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Expect Write() to be called
		writeCall := mockFile.Method.Write.ExpectCalledWithMatches(imptest.Any())
		writeCall.InjectReturnValues(9, nil)

		// Expect Close() and Release() from defer
		closeCall := mockFile.Method.Close.Eventually.ExpectCalledWithExactly()
		closeCall.InjectReturnValues(nil)

		releaseCall := mockPool.Method.Release.Eventually.ExpectCalledWithMatches(imptest.Any())
		releaseCall.InjectReturnValues()
	}()

	func() {
		pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
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

	<-done
	// Success means Write(), Close(), and Release() were all called, with defer working correctly
}

// TestPooledSFTPFile_Documentation_UsageExample documents expected usage.
func TestPooledSFTPFile_Documentation_UsageExample(t *testing.T) {
	t.Parallel()

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

// TestPooledSFTPFile_DoubleClose_IsSafe tests that Close is idempotent.
func TestPooledSFTPFile_DoubleClose_IsSafe(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Expect exactly one call to Close() and one to Release()
		// If called again, the expectations will fail
		closeCall := mockFile.Method.Close.ExpectCalledWithExactly()
		closeCall.InjectReturnValues(nil)

		releaseCall := mockPool.Method.Release.Eventually.ExpectCalledWithMatches(imptest.Any())
		releaseCall.InjectReturnValues()
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	// First close - should call file.Close() and pool.Release()
	err1 := pooledFile.Close()
	if err1 != nil {
		t.Errorf("First close failed: %v", err1)
	}

	// Second close - should be no-op (no additional calls)
	err2 := pooledFile.Close()
	if err2 != nil {
		t.Errorf("Second close should not error, got: %v", err2)
	}

	<-done
	// Success means only one Close() and one Release() were called
}

// TestPooledSFTPFile_ErrorDuringRead_ClientStillReleasedOnClose tests error handling.
func TestPooledSFTPFile_ErrorDuringRead_ClientStillReleasedOnClose(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read failed")
	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Expect Read() to be called and return error
		readCall := mockFile.Method.Read.ExpectCalledWithMatches(imptest.Any())
		readCall.InjectReturnValues(0, readErr)

		// Expect Close() and Release() to still be called despite read error
		closeCall := mockFile.Method.Close.Eventually.ExpectCalledWithExactly()
		closeCall.InjectReturnValues(nil)

		releaseCall := mockPool.Method.Release.Eventually.ExpectCalledWithMatches(imptest.Any())
		releaseCall.InjectReturnValues()
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	// Read fails
	_, err = pooledFile.Read(make([]byte, 10))
	if err != readErr { //nolint:errorlint // Test verifies exact error instance is returned, not error chain
		t.Errorf("Expected error %v, got %v", readErr, err)
	}

	// Close should still release client
	err = pooledFile.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	<-done
	// Success means Close() and Release() were called despite read error
}

// TestPooledSFTPFile_ImplementsFileInterface tests interface compliance.
func TestPooledSFTPFile_ImplementsFileInterface(t *testing.T) {
	t.Parallel()

	// This should compile if PooledSFTPFile implements filesystem.File
	var _ filesystem.File = (*filesystem.PooledSFTPFile)(nil)
}

// TestPooledSFTPFile_MultipleReadsWrites_WorkCorrectly tests normal operations.
//
//nolint:funlen // Comprehensive read/write operation testing with multiple scenarios
func TestPooledSFTPFile_MultipleReadsWrites_WorkCorrectly(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	buffer := []byte{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Expect first write ("hello ")
		write1 := mockFile.Method.Write.Eventually.ExpectCalledWithMatches(imptest.Any())
		args1 := write1.GetArgs()
		buffer = append(buffer, args1.P...)
		write1.InjectReturnValues(len(args1.P), nil)

		// Expect second write ("world")
		write2 := mockFile.Method.Write.Eventually.ExpectCalledWithMatches(imptest.Any())
		args2 := write2.GetArgs()
		buffer = append(buffer, args2.P...)
		write2.InjectReturnValues(len(args2.P), nil)

		// Expect read - copy buffer to the read buffer
		read1 := mockFile.Method.Read.Eventually.ExpectCalledWithMatches(imptest.Any())
		readArgs := read1.GetArgs()
		n := copy(readArgs.P, buffer)
		read1.InjectReturnValues(n, nil)

		// Expect Close from defer
		close1 := mockFile.Method.Close.Eventually.ExpectCalledWithExactly()
		close1.InjectReturnValues(nil)

		release1 := mockPool.Method.Release.Eventually.ExpectCalledWithMatches(imptest.Any())
		release1.InjectReturnValues()
	}()

	func() {
		pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
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
	}()

	<-done
}

// TestPooledSFTPFile_NilSafety_NilClient tests nil client handling.
func TestPooledSFTPFile_NilSafety_NilClient(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockPool := MockClientPool(t)

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, nil, mockPool.Mock)
	if err == nil {
		t.Error("Should error on nil client")
	}
	if pooledFile != nil {
		t.Error("Should return nil pooledFile on error")
	}
}

// TestPooledSFTPFile_NilSafety_NilFile tests nil file handling.
func TestPooledSFTPFile_NilSafety_NilFile(t *testing.T) {
	t.Parallel()

	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	pooledFile, err := filesystem.NewPooledSFTPFile(nil, mockClient, mockPool.Mock)
	if err == nil {
		t.Error("Should error on nil file")
	}
	if pooledFile != nil {
		t.Error("Should return nil pooledFile on error")
	}
}

// TestPooledSFTPFile_NilSafety_NilPool tests nil pool handling.
func TestPooledSFTPFile_NilSafety_NilPool(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, nil)
	if err == nil {
		t.Error("Should error on nil pool")
	}
	if pooledFile != nil {
		t.Error("Should return nil pooledFile on error")
	}
}

// TestPooledSFTPFile_ReadWrite_AfterClose_ReturnsError tests usage after close.
func TestPooledSFTPFile_ReadWrite_AfterClose_ReturnsError(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Only expect Close() to be called
		closeCall := mockFile.Method.Close.ExpectCalledWithExactly()
		closeCall.InjectReturnValues(nil)

		releaseCall := mockPool.Method.Release.Eventually.ExpectCalledWithMatches(imptest.Any())
		releaseCall.InjectReturnValues()
		// Read() and Write() should NOT be called - pooled file should short-circuit
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	pooledFile.Close()
	<-done

	// After close, Read() and Write() should return fs.ErrClosed without calling mocks
	_, readErr := pooledFile.Read(make([]byte, 10))
	if !errors.Is(readErr, fs.ErrClosed) {
		t.Errorf("Read after Close should return fs.ErrClosed, got: %v", readErr)
	}

	_, writeErr := pooledFile.Write([]byte("data"))
	if !errors.Is(writeErr, fs.ErrClosed) {
		t.Errorf("Write after Close should return fs.ErrClosed, got: %v", writeErr)
	}
}

// TestPooledSFTPFile_Read_DelegatesToWrappedFile tests that Read delegates to underlying file.
func TestPooledSFTPFile_Read_DelegatesToWrappedFile(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	var readCall *SftpFileMockReadCall
	done := make(chan struct{})
	go func() {
		defer close(done)
		readCall = mockFile.Method.Read.ExpectCalledWithMatches(imptest.Any())
		// Modify buffer (side effect) before returning
		args := readCall.GetArgs()
		copy(args.P, []byte("test data"))
		readCall.InjectReturnValues(9, nil)
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	buf := make([]byte, 100)
	n, err := pooledFile.Read(buf)
	<-done

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

// TestPooledSFTPFile_Stat_DelegatesToWrappedFile tests that Stat delegates to underlying file.
func TestPooledSFTPFile_Stat_DelegatesToWrappedFile(t *testing.T) {
	t.Parallel()

	mockFileInfo := MockFileInfo(t)
	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Set up file Stat expectation
		statCall := mockFile.Method.Stat.ExpectCalledWithExactly()
		statCall.InjectReturnValues(mockFileInfo.Mock, nil)

		// Set up FileInfo mock expectations (test calls Name and Size)
		mockFileInfo.Method.Name.Eventually.ExpectCalledWithExactly().InjectReturnValues("test.txt")
		mockFileInfo.Method.Size.Eventually.ExpectCalledWithExactly().InjectReturnValues(int64(1024))
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
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

	<-done
}

// TestPooledSFTPFile_Write_DelegatesToWrappedFile tests that Write delegates to underlying file.
func TestPooledSFTPFile_Write_DelegatesToWrappedFile(t *testing.T) {
	t.Parallel()

	mockFile := MockSftpFile(t)
	mockClient := &sftp.Client{}
	mockPool := MockClientPool(t)

	testData := []byte("test write")
	var writeCall *SftpFileMockWriteCall

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Accept any byte slice - we'll verify using GetArgs
		writeCall = mockFile.Method.Write.ExpectCalledWithMatches(imptest.Any())
		writeCall.InjectReturnValues(10, nil)
	}()

	pooledFile, err := filesystem.NewPooledSFTPFile(mockFile.Mock, mockClient, mockPool.Mock)
	if err != nil {
		t.Fatalf("NewPooledSFTPFile failed: %v", err)
	}

	n, err := pooledFile.Write(testData)
	<-done

	// Verify return values
	if err != nil {
		t.Errorf("Write should succeed, got error: %v", err)
	}
	if n != 10 {
		t.Errorf("Should write 10 bytes, got %d", n)
	}

	// Verify side effect - what data was actually written
	args := writeCall.GetArgs()
	if string(args.P) != string(testData) {
		t.Errorf("Expected %q, got %q", testData, args.P)
	}
}
