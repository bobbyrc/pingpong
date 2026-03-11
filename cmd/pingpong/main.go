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

	"github.com/bobbyrc/pingpong/internal/alerter"
	"github.com/bobbyrc/pingpong/internal/collector"
	"github.com/bobbyrc/pingpong/internal/config"
	"github.com/bobbyrc/pingpong/internal/metrics"
	"github.com/bobbyrc/pingpong/internal/web"
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
		"dns_targets", cfg.DNSTargets,
		"dns_servers", cfg.DNSServers,
		"dns_interval", cfg.DNSInterval,
		"speedtest_interval", cfg.SpeedtestInterval,
		"speedtest_server_id", cfg.SpeedtestServerID,
		"traceroute_interval", cfg.TracerouteInterval,
	)

	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	pingCollector := collector.NewPingCollector(cfg.PingTargets, cfg.PingCount)
	speedCollector := collector.NewSpeedtestCollector(cfg.SpeedtestServerID)
	dnsCollector := collector.NewDNSCollector(cfg.DNSTargets, cfg.DNSServers)
	traceCollector := collector.NewTracerouteCollector(cfg.TracerouteTarget)

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		slog.Error("failed to create data directory", "path", cfg.DataDir, "error", err)
		os.Exit(1)
	}
	db, err := alerter.OpenDB(filepath.Join(cfg.DataDir, "alerts.db"))
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	queue, err := alerter.NewQueue(db)
	if err != nil {
		slog.Error("failed to create alert queue", "error", err)
		os.Exit(1)
	}

	history, err := web.NewHistoryStore(db)
	if err != nil {
		slog.Warn("failed to create history store — continuing without history", "error", err)
		history = nil
	}

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

	// Web UI
	webHandler, err := web.NewHandler(reg, queue, history, cfg.EnvFile)
	if err != nil {
		slog.Error("failed to create web handler", "error", err)
		os.Exit(1)
	}
	webHandler.RegisterRoutes(mux)

	// Resolve hostnames for ping targets
	hostnames := collector.ResolveHostnames(cfg.PingTargets)
	for target, hostname := range hostnames {
		slog.Info("resolved hostname", "target", target, "hostname", hostname)
	}
	webHandler.SetHostnames(hostnames)

	// Core routes
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	webHandler.Start(ctx)

	var wg sync.WaitGroup

	var downSince time.Time
	flushCh := make(chan struct{}, 1)

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

			connState := engine.ConnState()
			if allDown && !connState.IsDown() {
				connState.SetDown(true)
				downSince = time.Now()
				m.ConnectionUp.Set(0)
				m.ConnectionFlaps.Inc()
				slog.Warn("connection detected as DOWN")
			} else if !allDown && connState.IsDown() {
				downtime := time.Since(downSince)
				m.DowntimeTotal.Add(downtime.Seconds())
				connState.SetDown(false)
				m.ConnectionUp.Set(1)
				m.ConnectionFlaps.Inc()
				slog.Info("connection restored", "downtime", downtime.Round(time.Second))
				select {
				case flushCh <- struct{}{}:
				default:
				}
			} else if !allDown {
				m.ConnectionUp.Set(1)
			}

			isDown := connState.IsDown()
			ds := downSince

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
				m.SpeedtestFailures.Inc()
				return
			}
			m.DownloadSpeed.Set(result.DownloadMbps)
			m.UploadSpeed.Set(result.UploadMbps)
			m.SpeedtestLatency.Set(result.LatencyMs)
			m.SpeedtestJitter.Set(result.JitterMs)

			m.SpeedtestInfo.Reset()
			if result.ServerName != "" || result.ISP != "" {
				m.SpeedtestInfo.WithLabelValues(result.ServerName, result.ServerLocation, result.ISP).Set(1)
			}

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
			results, failures := dnsCollector.Collect(ctx)
			for _, r := range results {
				m.DNSResolution.WithLabelValues(r.Target, r.Server).Set(r.ResolutionMs)
			}
			for _, f := range failures {
				m.DNSFailures.WithLabelValues(f.Target, f.Server).Inc()
			}
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
				m.TracerouteFailures.Inc()
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
			case <-flushCh:
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
