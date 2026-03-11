# Connection-Aware Alert Retry

## Problem

When the internet connection is down, `ProcessQueue()` runs on a configurable interval (default: 60s) and attempts to send all pending alerts via Apprise. Every attempt fails, incrementing `retry_count`. With default max retries of 30, alerts can hit `failed_permanent` in ~30 minutes — potentially before the connection recovers. This wastes retries on attempts we know will fail.

## Decision

Introduce a shared `ConnectionState` struct that lets `ProcessQueue()` skip send attempts while the connection is down. Alerts are still enqueued during outages (preserving a complete threshold-crossing history), but retries are paused until the connection is restored. On recovery, pending alerts are flushed immediately rather than waiting for the next retry tick.

## Design

### New type: `ConnectionState` in `internal/alerter/connstate.go`

A thread-safe wrapper around a boolean:

```go
type ConnectionState struct {
    mu   sync.RWMutex
    down bool
}

func NewConnectionState() *ConnectionState
func (cs *ConnectionState) SetDown(down bool)
func (cs *ConnectionState) IsDown() bool
```

Lives in the `alerter` package since that's where it's consumed. The zero value of `down` (`false`) means "not down," so retries proceed normally before the first ping completes — a safe default.

### Changes to `Engine`

- Add `connState *ConnectionState` field, owned by the engine.
- `NewEngine()` constructs its own `ConnectionState` instance and stores it in `connState`. Expose it via a `ConnState() *ConnectionState` accessor so that `main.go` can observe and update connection status.
- `ProcessQueue()` adds a guard at the top: if `connState != nil && connState.IsDown()`, log at debug level and return early. Retry counts are not incremented.

### Changes to `main.go`

- Obtain `connState := engine.ConnState()` from the engine instance.
- In the ping loop state machine, replace the local `connDown` bool writes with `connState.SetDown(true/false)`. Keep `downSince` as a local variable (only the ping loop uses it). Read `connState.IsDown()` where `connDown` was previously read under the mutex.
- Remove the local `connMu` and `connDown` variables (replaced by `connState`).
- On the UP transition (connection restored), signal the retry-loop goroutine via a buffered `flushCh` channel so it wakes immediately and calls `ProcessQueue()` without waiting for the next tick.

### What doesn't change

- Alert enqueueing during outages — full history preserved.
- Retry counts are not incremented while connection is down — alerts won't age out.
- The retry ticker keeps running (no-ops while down).
- All existing tests pass without modification (nil `connState` = current behavior).

### Test plan

- Unit test `ConnectionState`: verify zero value is not-down, `SetDown(true)` makes `IsDown()` return true, `SetDown(false)` restores it.
- Unit test `ProcessQueue` with connection down: verify it returns without calling `apprise.Send()`.
- Unit test `ProcessQueue` with connection up: verify existing retry behavior is unchanged.
- Integration: run `make check` to verify all existing tests still pass.
