package collector

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type TracerouteHop struct {
	Number    int
	Address   string
	LatencyMs float64
}

type TracerouteResult struct {
	Target   string
	HopCount int
	Hops     []TracerouteHop
}

var (
	hopLineRe = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)
	latencyRe = regexp.MustCompile(`([\d.]+)\s*ms`)
	addressRe = regexp.MustCompile(`\(([\d.]+)\)`)
	bareIPRe  = regexp.MustCompile(`(?:^|\s)([\d]+\.[\d]+\.[\d]+\.[\d]+)\s`)
)

func parseTracerouteOutput(target, output string) TracerouteResult {
	lines := strings.Split(output, "\n")
	result := TracerouteResult{Target: target}

	for _, line := range lines {
		match := hopLineRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		hopNum, _ := strconv.Atoi(match[1])
		rest := match[2]

		hop := TracerouteHop{Number: hopNum}

		if strings.TrimSpace(rest) == "* * *" {
			hop.Address = "*"
			result.Hops = append(result.Hops, hop)
			continue
		}

		addrMatch := addressRe.FindStringSubmatch(rest)
		if addrMatch != nil {
			hop.Address = addrMatch[1]
		} else if bareMatch := bareIPRe.FindStringSubmatch(rest); bareMatch != nil {
			hop.Address = bareMatch[1]
		}

		latMatches := latencyRe.FindAllStringSubmatch(rest, -1)
		if len(latMatches) > 0 {
			var sum float64
			for _, m := range latMatches {
				val, _ := strconv.ParseFloat(m[1], 64)
				sum += val
			}
			hop.LatencyMs = sum / float64(len(latMatches))
		}

		result.Hops = append(result.Hops, hop)
	}

	result.HopCount = len(result.Hops)
	return result
}

type TracerouteCollector struct {
	target string
}

func NewTracerouteCollector(target string) *TracerouteCollector {
	return &TracerouteCollector{target: target}
}

func (tr *TracerouteCollector) Collect(ctx context.Context) (TracerouteResult, error) {
	slog.Info("running traceroute", "target", tr.target)
	cmd := exec.CommandContext(ctx, "traceroute", "-n", "-w", "2", tr.target)
	output, err := cmd.Output()
	if err != nil {
		if len(output) == 0 {
			return TracerouteResult{}, fmt.Errorf("run traceroute: %w", err)
		}
	}

	result := parseTracerouteOutput(tr.target, string(output))
	slog.Info("traceroute complete", "target", tr.target, "hops", result.HopCount)
	return result, nil
}
