package shared_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/tui/shared"
)

func TestFormatBytes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test various byte sizes
	g.Expect(shared.FormatBytes(500)).Should(Equal("500 B"))
	g.Expect(shared.FormatBytes(1024)).Should(Equal("1.0 KB"))
	g.Expect(shared.FormatBytes(1024 * 1024)).Should(Equal("1.0 MB"))
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test various durations
	result := shared.FormatDuration(30 * 1e9) // 30 seconds in nanoseconds
	g.Expect(result).ShouldNot(BeEmpty())
}

func TestFormatRate(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test various rates
	g.Expect(shared.FormatRate(500)).Should(Equal("500 B/s"))
	g.Expect(shared.FormatRate(1024)).Should(ContainSubstring("KB/s"))
	g.Expect(shared.FormatRate(1024 * 1024)).Should(ContainSubstring("MB/s"))
}
