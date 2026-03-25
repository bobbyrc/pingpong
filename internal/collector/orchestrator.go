package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// TriggerReason describes why a bandwidth test was triggered.
type TriggerReason string

const (
	TriggerBaseline           TriggerReason = "baseline"
	TriggerLatencySpike       TriggerReason = "latency_spike"
	TriggerJitterSpike        TriggerReason = "jitter_spike"
	TriggerPacketLoss         TriggerReason = "packet_loss"
	TriggerDNSSlow            TriggerReason = "dns_slow"
	TriggerConnectionRecovery TriggerReason = "connection_recovery"
)

// TriggerEvent records a trigger occurrence.
type TriggerEvent struct {
	Reason TriggerReason
	Time   time.Time
}

// BandwidthResult contains the results of an orchestrated bandwidth test.
type BandwidthResult struct {
	NDT7        *NDT7Result
	Bufferbloat *BufferbloatResult
	Trigger     TriggerEvent
	NDT7Err     error // non-nil if NDT7 test was attempted but failed
	BloatErr    error // non-nil if bufferbloat test was attempted but failed
}

// OrchestratorConfig holds tunable parameters for the orchestrator.
type OrchestratorConfig struct {
	BaselineInterval   time.Duration // How often to run baseline tests (default 6h)
	MinNDT7Interval    time.Duration // Minimum time between NDT7 tests (default 4h)
	MinBloatInterval   time.Duration // Minimum time between bufferbloat tests (default 1h)
	TriggerCooldown    time.Duration // Minimum time between trigger-based tests (default 30m)
	WarmupSamples      int           // Samples before baselines are active (default 5)
	LatencyMultiplier  float64       // Trigger when latency exceeds baseline * this (default 2.0)
	JitterMultiplier   float64       // Trigger when jitter exceeds baseline * this (default 3.0)
	PacketLossThreshold float64      // Trigger when packet loss > this % (default 1.0)
	DNSMultiplier      float64       // Trigger when DNS exceeds baseline * this (default 2.0)
}

// DefaultOrchestratorConfig returns sensible defaults.
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		BaselineInterval:    6 * time.Hour,
		MinNDT7Interval:     4 * time.Hour,
		MinBloatInterval:    1 * time.Hour,
		TriggerCooldown:     30 * time.Minute,
		WarmupSamples:       5,
		LatencyMultiplier:   2.0,
		JitterMultiplier:    3.0,
		PacketLossThreshold: 1.0,
		DNSMultiplier:       2.0,
	}
}

// BandwidthOrchestrator watches for network anomalies and triggers bandwidth tests.
type BandwidthOrchestrator struct {
	ndt7       *NDT7Collector
	bufferbloat *BufferbloatCollector

	cfg OrchestratorConfig

	mu               sync.Mutex
	pingBaseline     *RollingAvg
	jitterBaseline   *RollingAvg
	dnsBaseline      *RollingAvg
	lastNDT7         time.Time
	lastBufferbloat  time.Time
	lastTrigger      time.Time

	// now is injectable for testing.
	now func() time.Time
}

// NewBandwidthOrchestrator creates an orchestrator.
func NewBandwidthOrchestrator(ndt7 *NDT7Collector, bb *BufferbloatCollector, cfg OrchestratorConfig) *BandwidthOrchestrator {
	return &BandwidthOrchestrator{
		ndt7:           ndt7,
		bufferbloat:    bb,
		cfg:            cfg,
		pingBaseline:   NewRollingAvg(0.1, cfg.WarmupSamples),
		jitterBaseline: NewRollingAvg(0.1, cfg.WarmupSamples),
		dnsBaseline:    NewRollingAvg(0.1, cfg.WarmupSamples),
		now:            time.Now,
	}
}

// Run starts the orchestrator loop. It runs baseline tests on schedule and
// listens for triggered tests via resultCh. Blocks until ctx is cancelled.
func (o *BandwidthOrchestrator) Run(ctx context.Context, resultCh chan<- BandwidthResult) {
	// Run initial baseline after a short delay
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
	}

	o.runTest(ctx, TriggerEvent{Reason: TriggerBaseline, Time: o.now()}, resultCh)

	if o.cfg.BaselineInterval <= 0 {
		slog.Warn("baseline interval is non-positive; disabling periodic baseline tests")
		<-ctx.Done()
		return
	}

	baselineTicker := time.NewTicker(o.cfg.BaselineInterval)
	defer baselineTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-baselineTicker.C:
			o.runTest(ctx, TriggerEvent{Reason: TriggerBaseline, Time: o.now()}, resultCh)
		}
	}
}

// ReportPing feeds ping results into the baseline tracker and may trigger
// a bandwidth test if anomalies are detected.
func (o *BandwidthOrchestrator) ReportPing(results []PingResult, triggerCh chan<- TriggerEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, r := range results {
		// Check for anomalies BEFORE updating baselines so spikes
		// are compared against the pre-spike baseline value.
		if o.pingBaseline.Ready() {
			if r.AvgMs > o.pingBaseline.Value()*o.cfg.LatencyMultiplier {
				o.maybeTrigger(TriggerLatencySpike, triggerCh)
			}
		}
		if o.jitterBaseline.Ready() {
			if r.JitterMs > o.jitterBaseline.Value()*o.cfg.JitterMultiplier {
				o.maybeTrigger(TriggerJitterSpike, triggerCh)
			}
		}
		if r.PacketLoss > o.cfg.PacketLossThreshold {
			o.maybeTrigger(TriggerPacketLoss, triggerCh)
		}

		// Update baselines after anomaly detection
		if r.PacketLoss < 100 && r.AvgMs > 0 {
			o.pingBaseline.Update(r.AvgMs)
		}
		if r.JitterMs > 0 {
			o.jitterBaseline.Update(r.JitterMs)
		}
	}
}

// ReportDNS feeds DNS resolution results and may trigger a bandwidth test.
func (o *BandwidthOrchestrator) ReportDNS(results []DNSResult, triggerCh chan<- TriggerEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, r := range results {
		// Check for anomalies BEFORE updating the baseline so spikes
		// are compared against the pre-spike baseline value.
		if o.dnsBaseline.Ready() {
			if r.ResolutionMs > o.dnsBaseline.Value()*o.cfg.DNSMultiplier {
				o.maybeTrigger(TriggerDNSSlow, triggerCh)
			}
		}

		if r.ResolutionMs > 0 {
			o.dnsBaseline.Update(r.ResolutionMs)
		}
	}
}

// ReportConnectionRecovery signals that the connection just came back up.
func (o *BandwidthOrchestrator) ReportConnectionRecovery(triggerCh chan<- TriggerEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.maybeTrigger(TriggerConnectionRecovery, triggerCh)
}

// maybeTrigger sends a trigger event if the cooldown has elapsed.
// Must be called with o.mu held.
func (o *BandwidthOrchestrator) maybeTrigger(reason TriggerReason, triggerCh chan<- TriggerEvent) {
	now := o.now()
	if now.Sub(o.lastTrigger) < o.cfg.TriggerCooldown {
		slog.Debug("trigger cooldown active", "reason", reason)
		return
	}
	select {
	case triggerCh <- TriggerEvent{Reason: reason, Time: now}:
		o.lastTrigger = now
		slog.Info("bandwidth test triggered", "reason", reason)
	default:
		slog.Debug("trigger channel full, skipping", "reason", reason)
	}
}

// runTest executes NDT7 and optionally bufferbloat tests, respecting minimum intervals.
func (o *BandwidthOrchestrator) runTest(ctx context.Context, trigger TriggerEvent, resultCh chan<- BandwidthResult) {
	result := BandwidthResult{
		Trigger: trigger,
	}

	o.mu.Lock()
	now := o.now()
	canNDT7 := now.Sub(o.lastNDT7) >= o.cfg.MinNDT7Interval
	canBloat := o.bufferbloat != nil && now.Sub(o.lastBufferbloat) >= o.cfg.MinBloatInterval
	// Reserve timestamps under the lock to prevent concurrent runTest calls
	// from both passing the interval check and running overlapping tests.
	if canNDT7 {
		o.lastNDT7 = now
	}
	if canBloat {
		o.lastBufferbloat = now
	}
	o.mu.Unlock()

	if canNDT7 {
		ndt7Result, err := o.ndt7.Collect(ctx)
		if err != nil {
			result.NDT7Err = err
			slog.Error("orchestrator NDT7 test failed", "reason", trigger.Reason, "error", err)
		} else {
			result.NDT7 = &ndt7Result
		}
	}

	if canBloat {
		bbResult, err := o.bufferbloat.Collect(ctx)
		if err != nil {
			result.BloatErr = err
			slog.Error("orchestrator bufferbloat test failed", "reason", trigger.Reason, "error", err)
		} else {
			result.Bufferbloat = &bbResult
		}
	}

	if canNDT7 || canBloat {
		select {
		case resultCh <- result:
		default:
			slog.Debug("result channel full, dropping bandwidth result")
		}
	}
}

// HandleTrigger processes a trigger event by running tests.
func (o *BandwidthOrchestrator) HandleTrigger(ctx context.Context, trigger TriggerEvent, resultCh chan<- BandwidthResult) {
	o.runTest(ctx, trigger, resultCh)
}
