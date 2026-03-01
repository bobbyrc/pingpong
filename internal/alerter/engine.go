package alerter

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bcraig/pingpong/internal/collector"
	"github.com/bcraig/pingpong/internal/config"
)

type Engine struct {
	queue   *Queue
	apprise *AppriseClient
	cfg     *config.Config

	mu            sync.Mutex
	lastAlertTime map[string]time.Time
}

func NewEngine(queue *Queue, apprise *AppriseClient, cfg *config.Config) *Engine {
	return &Engine{
		queue:         queue,
		apprise:       apprise,
		cfg:           cfg,
		lastAlertTime: make(map[string]time.Time),
	}
}

func (e *Engine) EvaluatePing(results []collector.PingResult) {
	for _, r := range results {
		if e.cfg.AlertPacketLossThreshold > 0 && r.PacketLoss >= e.cfg.AlertPacketLossThreshold {
			e.fireAlert("packet_loss",
				fmt.Sprintf("High Packet Loss: %s", r.Target),
				fmt.Sprintf("Packet loss to %s is %.1f%% (threshold: %.1f%%)",
					r.Target, r.PacketLoss, e.cfg.AlertPacketLossThreshold),
			)
		}

		if e.cfg.AlertPingThreshold > 0 && r.AvgMs >= e.cfg.AlertPingThreshold {
			e.fireAlert("latency",
				fmt.Sprintf("High Latency: %s", r.Target),
				fmt.Sprintf("Ping to %s is %.1fms (threshold: %.1fms)",
					r.Target, r.AvgMs, e.cfg.AlertPingThreshold),
			)
		}

		if e.cfg.AlertJitterThreshold > 0 && r.JitterMs >= e.cfg.AlertJitterThreshold {
			e.fireAlert("jitter",
				fmt.Sprintf("High Jitter: %s", r.Target),
				fmt.Sprintf("Jitter to %s is %.1fms (threshold: %.1fms)",
					r.Target, r.JitterMs, e.cfg.AlertJitterThreshold),
			)
		}
	}
}

func (e *Engine) EvaluateSpeed(result collector.SpeedtestResult) {
	if e.cfg.AlertSpeedThreshold > 0 && result.DownloadMbps < e.cfg.AlertSpeedThreshold {
		e.fireAlert("speed",
			"Slow Download Speed",
			fmt.Sprintf("Download speed is %.1f Mbps (threshold: %.1f Mbps)",
				result.DownloadMbps, e.cfg.AlertSpeedThreshold),
		)
	}
}

func (e *Engine) EvaluateDowntime(isDown bool, downSince time.Time) {
	if !isDown || e.cfg.AlertDowntimeThreshold == 0 {
		return
	}

	downtime := time.Since(downSince)
	if downtime >= e.cfg.AlertDowntimeThreshold {
		e.fireAlert("downtime",
			"Internet Connection Down",
			fmt.Sprintf("Connection has been down for %s (threshold: %s)",
				downtime.Round(time.Second), e.cfg.AlertDowntimeThreshold),
		)
	}
}

func (e *Engine) fireAlert(alertType, title, body string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if last, ok := e.lastAlertTime[alertType]; ok {
		if time.Since(last) < e.cfg.AlertCooldown {
			slog.Debug("alert cooldown active", "type", alertType)
			return
		}
	}

	if err := e.queue.Enqueue(alertType, title, body); err != nil {
		slog.Error("failed to enqueue alert", "type", alertType, "error", err)
		return
	}

	e.lastAlertTime[alertType] = time.Now()
	slog.Warn("alert fired", "type", alertType, "title", title)
}

func (e *Engine) ProcessQueue() {
	if e.apprise == nil {
		return
	}

	pending, err := e.queue.Pending()
	if err != nil {
		slog.Error("failed to get pending alerts", "error", err)
		return
	}

	for _, alert := range pending {
		err := e.apprise.Send(alert.Title, alert.Body)
		if err != nil {
			slog.Error("failed to send alert", "id", alert.ID, "error", err)
			e.queue.IncrementRetry(alert.ID)

			if alert.RetryCount+1 >= e.cfg.AlertMaxRetries {
				slog.Error("alert exceeded max retries, marking permanent failure",
					"id", alert.ID, "type", alert.AlertType)
				e.queue.MarkFailedPermanent(alert.ID)
			}
			continue
		}

		e.queue.MarkSent(alert.ID)
		slog.Info("alert sent successfully", "id", alert.ID, "type", alert.AlertType)
	}
}
