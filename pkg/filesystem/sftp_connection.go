package filesystem

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SFTPConnection holds an active SSH/SFTP connection.
type SFTPConnection struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	host       string
	port       int
	user       string
}

// Connect establishes an SSH connection and opens an SFTP session.
// It uses SSH agent and default SSH keys for authentication.
func Connect(host string, port int, user string) (*SFTPConnection, error) {
	// Get authentication methods
	authMethods, err := getSSHAuthMethods()
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH auth methods: %w", err)
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no SSH authentication methods available (tried SSH agent and default keys)")
	}

	// Create SSH client config
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Add proper host key verification
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", host, port)
	sshClient, err := ssh.Dial("tcp", addr, config)
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
		host:       host,
		port:       port,
		user:       user,
	}, nil
}

// Close closes the SFTP session and SSH connection.
func (c *SFTPConnection) Close() error {
	var firstErr error

	if c.sftpClient != nil {
		if err := c.sftpClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if c.sshClient != nil {
		if err := c.sshClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// Client returns the underlying SFTP client.
func (c *SFTPConnection) Client() *sftp.Client {
	return c.sftpClient
}

// SSHClient returns the underlying SSH client for advanced usage (e.g., client pooling).
func (c *SFTPConnection) SSHClient() *ssh.Client {
	return c.sshClient
}

// getSSHAuthMethods returns SSH authentication methods in priority order:
// 1. SSH agent
// 2. Default SSH keys
func getSSHAuthMethods() ([]ssh.AuthMethod, error) {
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

	return authMethods, nil
}

// trySSHAgent attempts to connect to the SSH agent.
func trySSHAgent() ssh.AuthMethod {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil
	}

	conn, err := net.Dial("unix", socket)
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

// tryDefaultSSHKeys tries to load SSH keys from default locations.
func tryDefaultSSHKeys() ([]ssh.AuthMethod, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sshDir := filepath.Join(homeDir, ".ssh")

	// Default key files to try (in order)
	keyFiles := []string{
		filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(sshDir, "id_rsa"),
		filepath.Join(sshDir, "id_ecdsa"),
	}

	var authMethods []ssh.AuthMethod

	for _, keyPath := range keyFiles {
		// Check if key file exists
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
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

// ReadSourceFileForTesting returns the contents of sftp_connection.go for test verification.
// This is used by tests to verify configuration options are set correctly in the source code.
func ReadSourceFileForTesting() string {
	// Get the current file's path
	_, filename, _, _ := runtime.Caller(0)

	// Read this file
	content, err := os.ReadFile(filename)
	if err != nil {
		return ""
	}

	return string(content)
}
