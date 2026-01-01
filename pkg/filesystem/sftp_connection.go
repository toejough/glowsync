package filesystem

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SFTPClientCloser defines the interface for SFTP client cleanup operations.
// This interface exists to enable testing of SFTPConnection.Close() using impgen-generated mocks.
//
//nolint:iface // Separate from SSHClientCloser for type safety in SFTPConnection struct fields
type SFTPClientCloser interface {
	Close() error
}

// SFTPConnection holds an active SSH/SFTP connection.
type SFTPConnection struct {
	sshClient  SSHClientCloser
	sftpClient SFTPClientCloser
}

// Client returns the underlying SFTP client.
// Panics if the client is not a *sftp.Client (should never happen in normal usage).
func (c *SFTPConnection) Client() *sftp.Client {
	if c.sftpClient == nil {
		return nil
	}

	return c.sftpClient.(*sftp.Client) //nolint:forcetypeassert,lll // Panic is intentional and documented; interface only holds *sftp.Client
}

// Close closes the SFTP session and SSH connection.
func (c *SFTPConnection) Close() error {
	var firstErr error

	if c.sftpClient != nil {
		if err := c.sftpClient.Close(); err != nil && firstErr == nil { //nolint:noinlineerr,lll // Inline error check is idiomatic for cleanup operations
			firstErr = err
		}
	}

	if c.sshClient != nil {
		if err := c.sshClient.Close(); err != nil && firstErr == nil { //nolint:noinlineerr,lll // Inline error check is idiomatic for cleanup operations
			firstErr = err
		}
	}

	return firstErr
}

// SSHClient returns the underlying SSH client for advanced usage (e.g., client pooling).
// Panics if the client is not a *ssh.Client (should never happen in normal usage).
func (c *SFTPConnection) SSHClient() *ssh.Client {
	if c.sshClient == nil {
		return nil
	}

	return c.sshClient.(*ssh.Client) //nolint:forcetypeassert,lll // Panic is intentional and documented; interface only holds *ssh.Client
}

// SSHClientCloser defines the interface for SSH client cleanup operations.
// This interface exists to enable testing of SFTPConnection.Close() using impgen-generated mocks.
//
//nolint:iface // Separate from SFTPClientCloser for type safety in SFTPConnection struct fields
type SSHClientCloser interface {
	Close() error
}

// SSHDialer defines the interface for establishing SSH connections.
// This interface enables testing by allowing SSH connection behavior to be mocked.
type SSHDialer interface {
	Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error)
}

// Connect establishes an SSH connection and opens an SFTP session.
// It uses SSH agent and default SSH keys for authentication.
func Connect(host string, port int, user string) (*SFTPConnection, error) {
	// Get authentication methods
	authMethods := getSSHAuthMethods()
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no SSH authentication methods available (tried SSH agent and default keys)") //nolint:err113,perfsprint,lll // Descriptive error for auth failure
	}

	// Create SSH client config
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // TODO: Add proper host key verification
		Timeout:         2 * time.Second,             //nolint:mnd // Connection timeout in seconds
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", host, port)
	sshClient, err := defaultSSHDialer.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}

	// Open SFTP session with concurrent writes enabled for better performance.
	// Note: Concurrent writes can create "holes" if writes fail mid-transfer.
	// Our error handling in CopyFileWithStats() mitigates this by deleting
	// partial files on error (see pkg/fileops/fileops.go:213-218).
	sftpClient, err := sftp.NewClient(sshClient, sftp.UseConcurrentWrites(true))
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("SFTP session creation failed: %w", err)
	}

	return &SFTPConnection{
		sshClient:  sshClient,
		sftpClient: sftpClient,
	}, nil
}

// ReadSourceFileForTesting returns the contents of sftp_connection.go for test verification.
// This is used by tests to verify configuration options are set correctly in the source code.
func ReadSourceFileForTesting() string {
	// Get the current file's path
	_, filename, _, _ := runtime.Caller(0) //nolint:dogsled,lll // Only need filename, other return values (pc, line, ok) intentionally ignored

	// Read this file
	content, err := os.ReadFile(filename)
	if err != nil {
		return ""
	}

	return string(content)
}

// SetSSHDialerForTesting allows tests to inject a mock SSH dialer.
// Returns a cleanup function that restores the original dialer.
// This should only be used in tests.
func SetSSHDialerForTesting(dialer SSHDialer) func() {
	old := defaultSSHDialer
	defaultSSHDialer = dialer

	return func() { defaultSSHDialer = old }
}

// unexported variables.
var (
	defaultSSHDialer SSHDialer = &realSSHDialer{} //nolint:gochecknoglobals // DI default
)

// realSSHDialer implements SSHDialer using the actual ssh.Dial function.
type realSSHDialer struct{}

func (d *realSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	return ssh.Dial(network, addr, config) //nolint:wrapcheck // External package error, wrapped by caller in Connect()
}

// getSSHAuthMethods returns SSH authentication methods in priority order:
// 1. SSH agent
// 2. Default SSH keys
func getSSHAuthMethods() []ssh.AuthMethod {
	var authMethods []ssh.AuthMethod

	// Try SSH agent first
	if agentAuth := trySSHAgent(); agentAuth != nil {
		authMethods = append(authMethods, agentAuth)
	}

	// Try default SSH keys
	keyAuths, err := tryDefaultSSHKeys()
	if err == nil && len(keyAuths) > 0 {
		authMethods = append(authMethods, keyAuths...)
	}

	return authMethods
}

// tryDefaultSSHKeys tries to load SSH keys from default locations.
func tryDefaultSSHKeys() ([]ssh.AuthMethod, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err //nolint:wrapcheck // Standard library error, wrapped by caller
	}

	sshDir := filepath.Join(homeDir, ".ssh")

	// Default key files to try (in order)
	keyFiles := []string{
		filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(sshDir, "id_rsa"),
		filepath.Join(sshDir, "id_ecdsa"),
	}

	var authMethods []ssh.AuthMethod //nolint:prealloc,lll // Cannot predict how many keys will be found, pre-allocation would be premature

	for _, keyPath := range keyFiles {
		// Check if key file exists
		if _, err := os.Stat(keyPath); os.IsNotExist(err) { //nolint:noinlineerr,lll // Inline error check is idiomatic for existence checks
			continue
		}

		// Read private key
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			continue
		}

		// Parse private key
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			// If the key is encrypted, skip it (we don't support password-protected keys)
			continue
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	return authMethods, nil
}

// trySSHAgent attempts to connect to the SSH agent.
func trySSHAgent() ssh.AuthMethod {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil
	}

	conn, err := net.Dial("unix", socket) //nolint:noctx,lll // Local Unix socket to SSH agent, no context available in this helper
	if err != nil {
		return nil
	}

	agentClient := agent.NewClient(conn)

	// Check if agent has any keys before using it
	signers, err := agentClient.Signers()
	if err != nil || len(signers) == 0 {
		return nil
	}

	return ssh.PublicKeysCallback(agentClient.Signers)
}
