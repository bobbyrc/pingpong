package alerter

import "testing"

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
