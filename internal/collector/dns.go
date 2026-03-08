package collector

import (
	"context"
	"log/slog"
	"net"
	"time"
)

type DNSResult struct {
	Target       string
	Server       string
	ResolutionMs float64
}

type DNSFailure struct {
	Target string
	Server string
	Err    error
}

type DNSCollector struct {
	targets   []string
	resolvers map[string]*net.Resolver // keyed by server name ("system", "1.1.1.1", etc)
}

func NewDNSCollector(targets []string, servers []string) *DNSCollector {
	resolvers := make(map[string]*net.Resolver)

	// Always include system resolver
	resolvers["system"] = net.DefaultResolver

	for _, server := range servers {
		s := server // capture for closure
		resolvers[s] = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, network, net.JoinHostPort(s, "53"))
			},
		}
	}

	return &DNSCollector{targets: targets, resolvers: resolvers}
}

func (d *DNSCollector) Collect(ctx context.Context) ([]DNSResult, []DNSFailure) {
	var results []DNSResult
	var failures []DNSFailure

	for _, target := range d.targets {
		for server, resolver := range d.resolvers {
			start := time.Now()
			_, err := resolver.LookupHost(ctx, target)
			elapsed := time.Since(start)

			if err != nil {
				slog.Error("dns lookup failed", "target", target, "server", server, "error", err)
				failures = append(failures, DNSFailure{Target: target, Server: server, Err: err})
				continue
			}

			results = append(results, DNSResult{
				Target:       target,
				Server:       server,
				ResolutionMs: float64(elapsed.Microseconds()) / 1000.0,
			})
		}
	}

	return results, failures
}
