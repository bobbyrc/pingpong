package alerter

import (
	"sync"
	"testing"
)

func TestConnectionState_ZeroValueIsNotDown(t *testing.T) {
	cs := NewConnectionState()
	if cs.IsDown() {
		t.Fatal("expected new ConnectionState to not be down")
	}
}

func TestConnectionState_SetDownTrue(t *testing.T) {
	cs := NewConnectionState()
	cs.SetDown(true)
	if !cs.IsDown() {
		t.Fatal("expected IsDown() to return true after SetDown(true)")
	}
}

func TestConnectionState_SetDownFalseRestores(t *testing.T) {
	cs := NewConnectionState()
	cs.SetDown(true)
	cs.SetDown(false)
	if cs.IsDown() {
		t.Fatal("expected IsDown() to return false after SetDown(false)")
	}
}

func TestConnectionState_ConcurrentAccess(t *testing.T) {
	cs := NewConnectionState()
	var wg sync.WaitGroup

	// Hammer SetDown/IsDown from multiple goroutines.
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			cs.SetDown(true)
			cs.SetDown(false)
		}()
		go func() {
			defer wg.Done()
			_ = cs.IsDown()
		}()
	}

	wg.Wait()
	// Final state should be consistent (last SetDown was false).
	if cs.IsDown() {
		t.Fatal("expected IsDown() to be false after all goroutines complete")
	}
}
