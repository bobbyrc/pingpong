package collector

// RollingAvg tracks an exponential moving average (EMA) with a configurable
// warmup period. It is safe for single-goroutine use.
type RollingAvg struct {
	alpha    float64
	minReady int
	count    int
	value    float64
}

// NewRollingAvg creates a RollingAvg with the given smoothing factor (alpha)
// and minimum number of samples before Ready() returns true.
func NewRollingAvg(alpha float64, minReady int) *RollingAvg {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.1
	}
	if minReady < 1 {
		minReady = 1
	}
	return &RollingAvg{alpha: alpha, minReady: minReady}
}

// Update adds a new sample to the EMA.
func (r *RollingAvg) Update(v float64) {
	r.count++
	if r.count == 1 {
		r.value = v
		return
	}
	r.value = r.alpha*v + (1-r.alpha)*r.value
}

// Ready returns true once at least minReady samples have been recorded.
func (r *RollingAvg) Ready() bool {
	return r.count >= r.minReady
}

// Value returns the current EMA value.
func (r *RollingAvg) Value() float64 {
	return r.value
}

// Count returns the number of samples recorded.
func (r *RollingAvg) Count() int {
	return r.count
}
