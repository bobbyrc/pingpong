# Connection-Aware Alert Retry Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent `ProcessQueue()` from burning retries while the internet connection is known to be down, and immediately flush pending alerts when the connection recovers.

**Architecture:** A shared `ConnectionState` struct in the `alerter` package provides thread-safe read/write of the connection-down boolean. `main.go` writes to it from the ping loop; `Engine.ProcessQueue()` reads it to short-circuit when down. On UP transition, `main.go` signals the existing retry-loop goroutine via a channel to flush immediately, avoiding concurrent `ProcessQueue()` calls.

**Tech Stack:** Go stdlib (`sync.RWMutex`), existing `alerter` and `main` packages.

**Spec:** `docs/superpowers/specs/2026-03-10-connection-aware-alert-retry-design.md`

---

## Chunk 1: ConnectionState + Engine Integration

### Task 1: Create `ConnectionState` with tests

**Files:**
- Create: `internal/alerter/connstate.go`
- Create: `internal/alerter/connstate_test.go`

- [ ] **Step 1: Write the failing tests**

In `internal/alerter/connstate_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/alerter/ -run TestConnectionState -v`
Expected: FAIL — `NewConnectionState` undefined.

- [ ] **Step 3: Write minimal implementation**

In `internal/alerter/connstate.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/alerter/ -run TestConnectionState -v`
Expected: PASS (3/3).

- [ ] **Step 5: Commit**

```bash
git add internal/alerter/connstate.go internal/alerter/connstate_test.go
git commit -m "feat: add ConnectionState for tracking connection up/down"
```

---

### Task 2: Wire `ConnectionState` into `Engine` and guard `ProcessQueue`

**Files:**
- Modify: `internal/alerter/engine.go` (struct, constructor, ProcessQueue)
- Modify: `internal/alerter/engine_test.go` (helper, new tests)

- [ ] **Step 1: Write the failing tests**

Add to `internal/alerter/engine_test.go`:

```go
func TestProcessQueue_SkipsWhenConnectionDown(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            1 * time.Second,
	})

	// Enqueue an alert.
	engine.EvaluatePing([]collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0},
	})

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert, got %d", len(pending))
	}

	// Mark connection as down.
	engine.connState.SetDown(true)

	// ProcessQueue should short-circuit — alert stays pending.
	engine.ProcessQueue()

	pending, _ = q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected alert to remain pending when connection is down, got %d", len(pending))
	}
	// Verify retry count was NOT incremented.
	if pending[0].RetryCount != 0 {
		t.Fatalf("expected retry count 0 (skipped), got %d", pending[0].RetryCount)
	}
}

func TestProcessQueue_ProceedsWhenConnectionUp(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            1 * time.Second,
	})

	// Enqueue an alert.
	engine.EvaluatePing([]collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0},
	})

	// Connection is up (default). ProcessQueue will attempt send
	// (which fails since dummyApprise points nowhere real), but
	// the point is it DOES attempt — retry count increments.
	engine.ProcessQueue()

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert, got %d", len(pending))
	}
	if pending[0].RetryCount != 1 {
		t.Fatalf("expected retry count 1 (attempted), got %d", pending[0].RetryCount)
	}
}

func TestProcessQueue_NilConnStateAlwaysProceeds(t *testing.T) {
	// Build engine without ConnectionState (simulates old behavior / tests).
	dir := t.TempDir()
	db, err := OpenDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	q, err := NewQueue(db)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}

	engine := NewEngine(q, dummyApprise, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            1 * time.Second,
	})
	// Explicitly set connState to nil to verify nil-safety.
	engine.connState = nil

	engine.EvaluatePing([]collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0},
	})

	// Should proceed (attempt send) even with nil connState.
	engine.ProcessQueue()

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert, got %d", len(pending))
	}
	if pending[0].RetryCount != 1 {
		t.Fatalf("expected retry count 1 (attempted), got %d", pending[0].RetryCount)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/alerter/ -run "TestProcessQueue_(Skips|Proceeds|NilConn)" -v`
Expected: FAIL — `engine.connState` undefined (field doesn't exist yet).

- [ ] **Step 3: Update `Engine` struct and constructor**

In `internal/alerter/engine.go`, add `connState` field to the struct and constructor:

```go
type Engine struct {
	queue     *Queue
	apprise   *AppriseClient
	cfg       *config.Config
	connState *ConnectionState

	mu            sync.Mutex
	lastAlertTime map[string]time.Time
}

func NewEngine(queue *Queue, apprise *AppriseClient, cfg *config.Config) *Engine {
	return &Engine{
		queue:         queue,
		apprise:       apprise,
		cfg:           cfg,
		connState:     NewConnectionState(),
		lastAlertTime: make(map[string]time.Time),
	}
}
```

- [ ] **Step 4: Add connection-down guard to `ProcessQueue`**

In `internal/alerter/engine.go`, add the guard at the top of `ProcessQueue()`, after the `apprise == nil` check:

```go
func (e *Engine) ProcessQueue() {
	if e.apprise == nil {
		return
	}

	if e.connState != nil && e.connState.IsDown() {
		slog.Debug("skipping alert retry, connection is down")
		return
	}

	// ... rest of existing ProcessQueue unchanged ...
```

- [ ] **Step 5: Add `ConnState()` accessor**

The `Engine` needs to expose `connState` so `main.go` can call `SetDown()`. Add a public accessor to `engine.go`:

```go
// ConnState returns the engine's connection state tracker.
func (e *Engine) ConnState() *ConnectionState {
	return e.connState
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/alerter/ -run "TestProcessQueue_(Skips|Proceeds|NilConn)" -v`
Expected: PASS (3/3).

- [ ] **Step 7: Run full test suite**

Run: `go test ./... -short`
Expected: All existing tests pass (constructor signature unchanged, `connState` defaults to non-nil).

- [ ] **Step 8: Commit**

```bash
git add internal/alerter/engine.go internal/alerter/engine_test.go
git commit -m "feat: guard ProcessQueue with ConnectionState check"
```

---

## Chunk 2: Wire into `main.go`

### Task 3: Replace local `connDown` with `engine.ConnState()`, add flush channel

**Files:**
- Modify: `cmd/pingpong/main.go` (ping loop state machine, retry loop, remove locals)

- [ ] **Step 1: Remove local `connMu`/`connDown` variables, add `flushCh`**

In `cmd/pingpong/main.go`, replace the local variable block (lines ~119-123):

```go
// REMOVE these lines:
var (
    connMu    sync.Mutex
    connDown  bool
    downSince time.Time
)
```

Replace with:

```go
var downSince time.Time
flushCh := make(chan struct{}, 1)
```

`connDown`/`connMu` are replaced by `engine.ConnState()`. `flushCh` is a buffered channel (capacity 1) used to signal the retry-loop goroutine to flush immediately on recovery. Buffered so the ping loop never blocks if the retry loop is mid-cycle.

- [ ] **Step 2: Update the ping loop state machine**

Replace the state machine block inside `runPing` (the `connMu.Lock()` through `connMu.Unlock()` section) with:

```go
			connState := engine.ConnState()
			if allDown && !connState.IsDown() {
				connState.SetDown(true)
				downSince = time.Now()
				m.ConnectionUp.Set(0)
				m.ConnectionFlaps.Inc()
				slog.Warn("connection detected as DOWN")
			} else if !allDown && connState.IsDown() {
				downtime := time.Since(downSince)
				m.DowntimeTotal.Add(downtime.Seconds())
				connState.SetDown(false)
				m.ConnectionUp.Set(1)
				m.ConnectionFlaps.Inc()
				slog.Info("connection restored", "downtime", downtime.Round(time.Second))
				select {
				case flushCh <- struct{}{}:
				default:
				}
			} else if !allDown {
				m.ConnectionUp.Set(1)
			}

			isDown := connState.IsDown()
			ds := downSince
```

Key changes from original:
- `connMu.Lock()/Unlock()` removed (mutex is internal to `ConnectionState`)
- `connDown` reads/writes become `connState.IsDown()`/`connState.SetDown()`
- Non-blocking send on `flushCh` on UP transition to wake the retry loop
- `downSince` stays as a local (only this goroutine uses it)

- [ ] **Step 3: Update the alert retry loop to select on `flushCh`**

Replace the existing alert retry loop goroutine (lines ~292-308) with:

```go
	// Alert retry loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.AlertRetryInterval)
		defer ticker.Stop()

		engine.ProcessQueue()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				engine.ProcessQueue()
			case <-flushCh:
				engine.ProcessQueue()
			}
		}
	}()
```

This ensures all `ProcessQueue()` calls are serialized through a single goroutine — no concurrent sends.

- [ ] **Step 4: Build to verify compilation**

Run: `go build ./cmd/pingpong/`
Expected: Build succeeds.

- [ ] **Step 5: Run full test suite**

Run: `go test ./... -short`
Expected: All tests pass.

- [ ] **Step 6: Run quality gate**

Run: `make check`
Expected: vet + test + tidy all pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/pingpong/main.go
git commit -m "feat: wire ConnectionState into ping loop, flush on recovery"
```
