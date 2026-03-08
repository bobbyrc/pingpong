package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/bcraig/pingpong/internal/alerter"
	"github.com/bcraig/pingpong/internal/collector"
	"github.com/bcraig/pingpong/internal/config"
	"github.com/bcraig/pingpong/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()
	slog.Info("pingpong starting",
		"ping_targets", cfg.PingTargets,
		"ping_interval", cfg.PingInterval,
		"speedtest_interval", cfg.SpeedtestInterval,
		"dns_interval", cfg.DNSInterval,
		"traceroute_interval", cfg.TracerouteInterval,
	)

	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	pingCollector := collector.NewPingCollector(cfg.PingTargets, cfg.PingCount)
	speedCollector := collector.NewSpeedtestCollector(cfg.SpeedtestServerID)
	dnsCollector := collector.NewDNSCollector(cfg.DNSTarget, cfg.DNSServer)
	traceCollector := collector.NewTracerouteCollector(cfg.TracerouteTarget)

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		slog.Error("failed to create data directory", "path", cfg.DataDir, "error", err)
		os.Exit(1)
	}
	queue, err := alerter.NewQueue(filepath.Join(cfg.DataDir, "alerts.db"))
	if err != nil {
		slog.Error("failed to open alert queue", "error", err)
		os.Exit(1)
	}
	defer queue.Close()

	var appriseClient *alerter.AppriseClient
	if cfg.AppriseURLs != "" {
		appriseClient = alerter.NewAppriseClient(cfg.AppriseURL, cfg.AppriseURLs)
		slog.Info("apprise notifications enabled")
	} else {
		slog.Warn("apprise not configured — notifications disabled")
	}

	engine := alerter.NewEngine(queue, appriseClient, cfg)
	engine.SeedCooldowns()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	var (
		connMu    sync.Mutex
		connDown  bool
		downSince time.Time
	)

	// Ping loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.PingInterval)
		defer ticker.Stop()

		runPing := func() {
			results := pingCollector.Collect(ctx)
			for _, r := range results {
				m.PingLatency.WithLabelValues(r.Target).Set(r.AvgMs)
				m.PingMin.WithLabelValues(r.Target).Set(r.MinMs)
				m.PingMax.WithLabelValues(r.Target).Set(r.MaxMs)
				m.Jitter.WithLabelValues(r.Target).Set(r.JitterMs)
				m.PacketLoss.WithLabelValues(r.Target).Set(r.PacketLoss)
			}

			allDown := len(results) > 0
			for _, r := range results {
				if r.PacketLoss < 100 {
					allDown = false
					break
				}
			}

			connMu.Lock()
			if allDown && !connDown {
				connDown = true
				downSince = time.Now()
				m.ConnectionUp.Set(0)
				slog.Warn("connection detected as DOWN")
			} else if !allDown && connDown {
				downtime := time.Since(downSince)
				m.DowntimeTotal.Add(downtime.Seconds())
				connDown = false
				m.ConnectionUp.Set(1)
				slog.Info("connection restored", "downtime", downtime.Round(time.Second))
			} else if !allDown {
				m.ConnectionUp.Set(1)
			}
			isDown := connDown
			ds := downSince
			connMu.Unlock()

			engine.EvaluatePing(results)
			if isDown {
				engine.EvaluateDowntime(true, ds)
			}
		}

		runPing()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runPing()
			}
		}
	}()

	// Speedtest loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.SpeedtestInterval)
		defer ticker.Stop()

		runSpeed := func() {
			result, err := speedCollector.Collect(ctx)
			if err != nil {
				slog.Error("speedtest failed", "error", err)
				return
			}
			m.DownloadSpeed.Set(result.DownloadMbps)
			m.UploadSpeed.Set(result.UploadMbps)
			m.SpeedtestLatency.Set(result.LatencyMs)

			engine.EvaluateSpeed(result)
		}

		runSpeed()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runSpeed()
			}
		}
	}()

	// DNS loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.DNSInterval)
		defer ticker.Stop()

		runDNS := func() {
			result, err := dnsCollector.Collect(ctx)
			if err != nil {
				slog.Error("DNS check failed", "error", err)
				return
			}
			m.DNSResolution.WithLabelValues(result.Target).Set(result.ResolutionMs)
		}

		runDNS()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runDNS()
			}
		}
	}()

	// Traceroute loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.TracerouteInterval)
		defer ticker.Stop()

		runTrace := func() {
			result, err := traceCollector.Collect(ctx)
			if err != nil {
				slog.Error("traceroute failed", "error", err)
				return
			}
			m.TracerouteHops.WithLabelValues(result.Target).Set(float64(result.HopCount))
			m.TracerouteHopLatency.Reset()
			for _, hop := range result.Hops {
				if hop.Address != "*" {
					m.TracerouteHopLatency.WithLabelValues(
						result.Target,
						fmt.Sprintf("%d", hop.Number),
						hop.Address,
					).Set(hop.LatencyMs)
				}
			}
		}

		runTrace()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runTrace()
			}
		}
	}()

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
			}
		}
	}()

	// HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("HTTP server starting", "addr", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	<-sigCh
	slog.Info("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)

	wg.Wait()
	slog.Info("pingpong stopped")
}
