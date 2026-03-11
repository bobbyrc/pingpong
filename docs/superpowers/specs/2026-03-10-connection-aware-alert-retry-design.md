# Connection-Aware Alert Retry

## Problem

When the internet connection is down, `ProcessQueue()` runs every 30 seconds and attempts to send all pending alerts via Apprise. Every attempt fails, incrementing `retry_count`. With a max of 100 retries at 30-second intervals, alerts can hit `failed_permanent` in ~50 minutes — potentially before the connection recovers. This wastes retries on attempts we know will fail.

## Decision

Introduce a shared `ConnectionState` struct that lets `ProcessQueue()` skip send attempts while the connection is down. Alerts are still enqueued during outages (preserving a complete threshold-crossing history), but retries are paused until the connection is restored. On recovery, pending alerts are flushed immediately rather than waiting for the next 30-second tick.

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

- Add `connState *ConnectionState` field.
- `NewEngine()` accepts `*ConnectionState` (nil-safe: if nil, retries always proceed, preserving current behavior for existing tests).
- `ProcessQueue()` adds a guard at the top: if `connState != nil && connState.IsDown()`, log at debug level and return early. Retry counts are not incremented.

### Changes to `main.go`

- Create `connState := alerter.NewConnectionState()` and pass it to `NewEngine()`.
- In the ping loop state machine, replace the local `connDown` bool writes with `connState.SetDown(true/false)`. Keep `downSince` as a local variable (only the ping loop uses it). Read `connState.IsDown()` where `connDown` was previously read under the mutex.
- Remove the local `connMu` and `connDown` variables (replaced by `connState`).
- On the UP transition (connection restored), call `go engine.ProcessQueue()` to immediately flush pending alerts.

### What doesn't change

- Alert enqueueing during outages — full history preserved.
- Retry counts are not incremented while connection is down — alerts won't age out.
- The 30-second retry ticker keeps running (no-ops while down).
- All existing tests pass without modification (nil `connState` = current behavior).

### Test plan

- Unit test `ConnectionState`: verify zero value is not-down, `SetDown(true)` makes `IsDown()` return true, `SetDown(false)` restores it.
- Unit test `ProcessQueue` with connection down: verify it returns without calling `apprise.Send()`.
- Unit test `ProcessQueue` with connection up: verify existing retry behavior is unchanged.
- Integration: run `make check` to verify all existing tests still pass.
