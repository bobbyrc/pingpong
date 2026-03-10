package collector

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net"
	"strings"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

type PingResult struct {
	Target     string
	AvgMs      float64
	MinMs      float64
	MaxMs      float64
	JitterMs   float64
	PacketLoss float64
}

func calculatePingResult(target string, rtts []time.Duration, sent int, lost int) PingResult {
	packetLoss := 100.0
	if sent > 0 {
		packetLoss = float64(lost) / float64(sent) * 100
	}

	result := PingResult{
		Target:     target,
		PacketLoss: packetLoss,
	}

	if len(rtts) == 0 {
		return result
	}

	var sum float64
	min := float64(rtts[0].Microseconds()) / 1000.0
	max := min

	for _, rtt := range rtts {
		ms := float64(rtt.Microseconds()) / 1000.0
		sum += ms
		if ms < min {
			min = ms
		}
		if ms > max {
			max = ms
		}
	}

	avg := sum / float64(len(rtts))
	result.AvgMs = avg
	result.MinMs = min
	result.MaxMs = max

	var varianceSum float64
	for _, rtt := range rtts {
		ms := float64(rtt.Microseconds()) / 1000.0
		diff := ms - avg
		varianceSum += diff * diff
	}
	result.JitterMs = math.Sqrt(varianceSum / float64(len(rtts)))

	return result
}

type PingCollector struct {
	targets []string
	count   int
}

func NewPingCollector(targets []string, count int) *PingCollector {
	return &PingCollector{targets: targets, count: count}
}

func (p *PingCollector) Collect(ctx context.Context) []PingResult {
	results := make([]PingResult, 0, len(p.targets))
	for _, target := range p.targets {
		result, err := p.ping(ctx, target)
		if err != nil {
			slog.Error("ping failed", "target", target, "error", err)
			results = append(results, PingResult{
				Target:     target,
				PacketLoss: 100,
			})
			continue
		}
		results = append(results, result)
	}
	return results
}

func (p *PingCollector) ping(ctx context.Context, target string) (PingResult, error) {
	pinger, err := probing.NewPinger(target)
	if err != nil {
		return PingResult{}, fmt.Errorf("create pinger: %w", err)
	}

	pinger.Count = p.count
	pinger.Timeout = time.Duration(p.count) * 2 * time.Second
	pinger.SetPrivileged(true)

	err = pinger.RunWithContext(ctx)
	if err != nil {
		return PingResult{}, fmt.Errorf("run ping: %w", err)
	}

	stats := pinger.Statistics()
	return calculatePingResult(
		target,
		stats.Rtts,
		stats.PacketsSent,
		stats.PacketsSent-stats.PacketsRecv,
	), nil
}

// ResolveHostnames performs reverse DNS lookups for each target and returns
// a map of target -> hostname. Targets that are already hostnames (not IPs)
// are mapped to themselves. If lookup fails, the target is omitted.
// Each lookup has a 2-second timeout to avoid blocking startup.
func ResolveHostnames(targets []string) map[string]string {
	result := make(map[string]string, len(targets))
	resolver := net.DefaultResolver
	for _, target := range targets {
		ip := net.ParseIP(target)
		if ip == nil {
			// Already a hostname
			result[target] = target
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		names, err := resolver.LookupAddr(ctx, target)
		cancel()
		if err != nil || len(names) == 0 {
			continue
		}
		// LookupAddr returns FQDNs with trailing dot; trim it
		hostname := strings.TrimRight(names[0], ".")
		if hostname != "" {
			result[target] = hostname
		}
	}
	return result
}
