package collector

import (
	"context"
	"fmt"
	"net"
	"time"
)

type DNSResult struct {
	Target       string
	ResolutionMs float64
}

type DNSCollector struct {
	target   string
	resolver *net.Resolver
}

func NewDNSCollector(target, server string) *DNSCollector {
	var resolver *net.Resolver
	if server != "" {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, network, net.JoinHostPort(server, "53"))
			},
		}
	} else {
		resolver = net.DefaultResolver
	}
	return &DNSCollector{target: target, resolver: resolver}
}

func (d *DNSCollector) Collect(ctx context.Context) (DNSResult, error) {
	start := time.Now()
	_, err := d.resolver.LookupHost(ctx, d.target)
	elapsed := time.Since(start)

	if err != nil {
		return DNSResult{}, fmt.Errorf("dns lookup %s: %w", d.target, err)
	}

	return DNSResult{
		Target:       d.target,
		ResolutionMs: float64(elapsed.Microseconds()) / 1000.0,
	}, nil
}
