package filesystem

import (
	"errors"
	"os"
	"testing"
)

// TestSFTPConnection_Client_ReturnsNilWhenNil tests Client() with nil sftpClient.
func TestSFTPConnection_Client_ReturnsNilWhenNil(t *testing.T) {
	t.Parallel()

	conn := &SFTPConnection{
		sftpClient: nil,
	}

	result := conn.Client()
	if result != nil {
		t.Errorf("Client() should return nil when sftpClient is nil, got %v", result)
	}
}

// Note: Client() and SSHClient() getter methods with non-nil clients cannot be tested
// with imptest-generated mocks because they perform type assertions to concrete types
// (*sftp.Client and *ssh.Client). These methods are designed for production use with
// real clients and achieve 66.7% coverage (nil paths tested). Full coverage would require
// integration tests with real SSH connections.

// TestSFTPConnection_Close_AfterSuccessfulConnection tests Close on a real connection.
// This is an integration test that requires SSH access. If SSH is not available,
// the test is skipped. This test achieves coverage for the actual Close() calls
// on real ssh.Client and sftp.Client instances.
func TestSFTPConnection_Close_AfterSuccessfulConnection(t *testing.T) {
	t.Parallel()

	// Check if we should skip SSH tests
	if os.Getenv("SKIP_SSH_TESTS") != "" {
		t.Skip("Skipping SSH integration test (SKIP_SSH_TESTS is set)")
	}

	// Try to connect to localhost - this will only work if SSH is running locally
	// and configured with agent/key auth
	conn, err := Connect("localhost", 22, os.Getenv("USER"))
	if err != nil {
		// If connection fails, we can't test Close() with real clients
		// This is expected in CI/environments without SSH
		t.Skipf("Cannot test Close() - SSH connection unavailable: %v", err)
		return
	}

	// If we got a connection, Close() should succeed without error
	err = conn.Close()
	if err != nil {
		t.Errorf("Close() should succeed after successful connection, got error: %v", err)
	}

	// Calling Close() again should be safe (idempotent)
	err = conn.Close()
	if err != nil {
		t.Logf("Second Close() returned error (expected for closed clients): %v", err)
	}
}

// TestSFTPConnection_Close_BothClientsSucceed tests Close when both clients close successfully.
func TestSFTPConnection_Close_BothClientsSucceed(t *testing.T) {
	t.Parallel()

	mockSSH := MockSSHClientCloser(t)
	mockSFTP := MockSFTPClientCloser(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		mockSFTP.Method.Close.ExpectCalledWithExactly().InjectReturnValues(nil)
		mockSSH.Method.Close.ExpectCalledWithExactly().InjectReturnValues(nil)
	}()

	conn := &SFTPConnection{
		sshClient:  mockSSH.Mock,
		sftpClient: mockSFTP.Mock,
	}

	err := conn.Close()
	<-done

	if err != nil {
		t.Errorf("Close should return nil when both clients succeed, got %v", err)
	}
}

// TestSFTPConnection_Close_BothFail tests that Close returns the first error when both fail.
func TestSFTPConnection_Close_BothFail(t *testing.T) {
	t.Parallel()

	mockSSH := MockSSHClientCloser(t)
	mockSFTP := MockSFTPClientCloser(t)

	sftpErr := errors.New("sftp close failed")
	sshErr := errors.New("ssh close failed")

	done := make(chan struct{})
	go func() {
		defer close(done)
		mockSFTP.Method.Close.ExpectCalledWithExactly().InjectReturnValues(sftpErr)
		mockSSH.Method.Close.ExpectCalledWithExactly().InjectReturnValues(sshErr)
	}()

	conn := &SFTPConnection{
		sshClient:  mockSSH.Mock,
		sftpClient: mockSFTP.Mock,
	}

	err := conn.Close()
	<-done

	// Should return first error (SFTP closes first)
	if err != sftpErr { //nolint:errorlint // Test verifies exact error instance is returned, not error chain
		t.Errorf("Close should return first error (SFTP), got %v", err)
	}
}

// TestSFTPConnection_Close_SFTPClientFails tests Close when SFTP client fails.
func TestSFTPConnection_Close_SFTPClientFails(t *testing.T) {
	t.Parallel()

	mockSSH := MockSSHClientCloser(t)
	mockSFTP := MockSFTPClientCloser(t)

	sftpErr := errors.New("sftp close failed")

	done := make(chan struct{})
	go func() {
		defer close(done)
		mockSFTP.Method.Close.ExpectCalledWithExactly().InjectReturnValues(sftpErr)
		mockSSH.Method.Close.ExpectCalledWithExactly().InjectReturnValues(nil)
	}()

	conn := &SFTPConnection{
		sshClient:  mockSSH.Mock,
		sftpClient: mockSFTP.Mock,
	}

	err := conn.Close()
	<-done

	if err != sftpErr { //nolint:errorlint // Test verifies exact error instance is returned, not error chain
		t.Errorf("Close should return SFTP error, got %v", err)
	}
}

// TestSFTPConnection_Close_SSHClientFails tests Close when SSH client fails.
func TestSFTPConnection_Close_SSHClientFails(t *testing.T) {
	t.Parallel()

	mockSSH := MockSSHClientCloser(t)
	mockSFTP := MockSFTPClientCloser(t)

	sshErr := errors.New("ssh close failed")

	done := make(chan struct{})
	go func() {
		defer close(done)
		mockSFTP.Method.Close.ExpectCalledWithExactly().InjectReturnValues(nil)
		mockSSH.Method.Close.ExpectCalledWithExactly().InjectReturnValues(sshErr)
	}()

	conn := &SFTPConnection{
		sshClient:  mockSSH.Mock,
		sftpClient: mockSFTP.Mock,
	}

	err := conn.Close()
	<-done

	if err != sshErr { //nolint:errorlint // Test verifies exact error instance is returned, not error chain
		t.Errorf("Close should return SSH error, got %v", err)
	}
}

//go:generate impgen --dependency filesystem.SSHClientCloser
//go:generate impgen --dependency filesystem.SFTPClientCloser

// TestSFTPConnection_Close_WithNilClients tests that Close handles nil clients gracefully.
func TestSFTPConnection_Close_WithNilClients(t *testing.T) {
	t.Parallel()

	conn := &SFTPConnection{
		sshClient:  nil,
		sftpClient: nil,
	}

	err := conn.Close()
	if err != nil {
		t.Errorf("Close should return nil for nil clients, got %v", err)
	}
}

// TestSFTPConnection_SSHClient_ReturnsNilWhenNil tests SSHClient() with nil sshClient.
func TestSFTPConnection_SSHClient_ReturnsNilWhenNil(t *testing.T) {
	t.Parallel()

	conn := &SFTPConnection{
		sshClient: nil,
	}

	result := conn.SSHClient()
	if result != nil {
		t.Errorf("SSHClient() should return nil when sshClient is nil, got %v", result)
	}
}
