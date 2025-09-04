package scheduler

import (
	"sync"
	"time"
)

// MockClock provides a mockable clock for testing
type MockClock struct {
	mu      sync.RWMutex
	now     time.Time
	waiting map[time.Duration][]chan time.Time
}

// NewMockClock creates a new MockClock with the given initial time
func NewMockClock(initialTime time.Time) *MockClock {
	return &MockClock{
		now:     initialTime,
		waiting: make(map[time.Duration][]chan time.Time),
	}
}

// Now returns the current mocked time
func (m *MockClock) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.now
}

// Since returns the duration since the given time
func (m *MockClock) Since(t time.Time) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.now.Sub(t)
}

// After returns a channel that will receive a value after the given duration
// For testing, this waits until the clock is advanced by the required duration
func (m *MockClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)

	if d <= 0 {
		// If duration is 0 or negative, send immediately
		ch <- m.Now()
		close(ch)
		return ch
	}

	// Store the channel to be signaled when Advance is called
	m.mu.Lock()
	if m.waiting[d] == nil {
		m.waiting[d] = make([]chan time.Time, 0)
	}
	m.waiting[d] = append(m.waiting[d], ch)
	m.mu.Unlock()

	return ch
}

// Advance advances the clock by the given duration and signals any waiting After calls
func (m *MockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldTime := m.now
	m.now = m.now.Add(d)

	// Signal any waiting After calls that should have fired
	for duration, channels := range m.waiting {
		if duration <= d {
			// All channels waiting for this duration or less should fire
			for _, ch := range channels {
				// Send the time value and close the channel
				ch <- oldTime.Add(duration)
				close(ch)
			}
			delete(m.waiting, duration)
		} else {
			// Reduce the remaining duration for channels waiting longer
			newDuration := duration - d
			if newDuration <= 0 {
				// These should also fire
				for _, ch := range channels {
					ch <- m.now
					close(ch)
				}
				delete(m.waiting, duration)
			} else {
				// Update the duration
				m.waiting[newDuration] = channels
				delete(m.waiting, duration)
			}
		}
	}
}

// SetTime sets the clock to a specific time
func (m *MockClock) SetTime(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = t
}

// GetTime returns the current mocked time (alias for Now for clarity)
func (m *MockClock) GetTime() time.Time {
	return m.Now()
}
