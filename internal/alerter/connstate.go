package alerter

import "sync"

// ConnectionState tracks whether the internet connection is down.
// The zero value (down=false) means "not down" — a safe default that
// allows retries to proceed before the first ping completes.
type ConnectionState struct {
	mu   sync.RWMutex
	down bool
}

func NewConnectionState() *ConnectionState {
	return &ConnectionState{}
}

func (cs *ConnectionState) SetDown(down bool) {
	cs.mu.Lock()
	cs.down = down
	cs.mu.Unlock()
}

func (cs *ConnectionState) IsDown() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.down
}
