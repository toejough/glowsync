package syncengine

import "time"

// MockTicker is a mock implementation of Ticker for testing.
type MockTicker struct {
	TickChan chan time.Time
}

// C returns the ticker's channel.
func (m *MockTicker) C() <-chan time.Time {
	return m.TickChan
}

// Stop stops the ticker.
func (m *MockTicker) Stop() {
	if m.TickChan != nil {
		close(m.TickChan)
	}
}

// RealTicker wraps time.Ticker to implement the Ticker interface.
type RealTicker struct {
	ticker *time.Ticker
}

// C returns the ticker's channel.
func (r *RealTicker) C() <-chan time.Time {
	return r.ticker.C
}

// Stop stops the ticker.
func (r *RealTicker) Stop() {
	r.ticker.Stop()
}

// RealTimeProvider implements TimeProvider using real time functions.
type RealTimeProvider struct{}

// NewTicker creates a new ticker.
func (r *RealTimeProvider) NewTicker(d time.Duration) Ticker {
	return &RealTicker{ticker: time.NewTicker(d)}
}

// Now returns the current time.
func (r *RealTimeProvider) Now() time.Time {
	return time.Now()
}

// Ticker is an interface for time.Ticker to allow mocking.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// TimeProvider provides time-related functionality for dependency injection.
type TimeProvider interface {
	Now() time.Time
	NewTicker(d time.Duration) Ticker
}
