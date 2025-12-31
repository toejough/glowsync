//nolint:varnamelen // Test files use idiomatic short variable names (t, etc.)
package fileops_test

import (
	"testing"

	"github.com/joe/copy-files/pkg/fileops"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers
)

// TestBufferSize_Is64KB verifies that BufferSize constant is 64KB.
// This test will FAIL until Phase 1.1 increases buffer size from 32KB to 64KB.
func TestBufferSize_Is64KB(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	expected := 64 * 1024 // 64KB

	g.Expect(fileops.BufferSize).Should(Equal(expected),
		"BufferSize should be 64KB (65536 bytes) for improved SFTP performance")
}

// TestMaxPacketSize_Exists_And_Is64KB is skipped because MaxPacketSize was removed.
// SFTP now uses default 32KB packet size for server compatibility.
// BufferSize remains at 64KB for local buffering performance.
func TestMaxPacketSize_Exists_And_Is64KB(t *testing.T) {
	t.Skip("MaxPacketSize removed - SFTP uses default 32KB packets for compatibility")
}

// TestBufferSize_And_MaxPacketSize_AreEqual is skipped because MaxPacketSize was removed.
// SFTP now uses default 32KB packet size for server compatibility.
// BufferSize remains at 64KB for local buffering performance.
func TestBufferSize_And_MaxPacketSize_AreEqual(t *testing.T) {
	t.Skip("MaxPacketSize removed - SFTP uses default 32KB packets for compatibility")
}
