package collector

// rollingAvg tracks an exponential moving average (EMA) with a configurable
// warmup period. It is safe for single-goroutine use.
type rollingAvg struct {
	alpha    float64
	minReady int
	n        int
	ema      float64
}

// newRollingAvg creates a rollingAvg with the given smoothing factor (alpha)
// and minimum number of samples before ready() returns true.
func newRollingAvg(alpha float64, minReady int) *rollingAvg {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.1
	}
	if minReady < 1 {
		minReady = 1
	}
	return &rollingAvg{alpha: alpha, minReady: minReady}
}

// update adds a new sample to the EMA.
func (r *rollingAvg) update(v float64) {
	r.n++
	if r.n == 1 {
		r.ema = v
		return
	}
	r.ema = r.alpha*v + (1-r.alpha)*r.ema
}

// ready returns true once at least minReady samples have been recorded.
func (r *rollingAvg) ready() bool {
	return r.n >= r.minReady
}

// value returns the current EMA value.
func (r *rollingAvg) value() float64 {
	return r.ema
}

// count returns the number of samples recorded.
func (r *rollingAvg) count() int {
	return r.n
}
