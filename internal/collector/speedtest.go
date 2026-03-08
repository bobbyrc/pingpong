package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
)

type SpeedtestResult struct {
	DownloadMbps   float64
	UploadMbps     float64
	LatencyMs      float64
	ServerName     string
	ServerLocation string
	ISP            string
}

type speedtestJSON struct {
	Ping struct {
		Latency float64 `json:"latency"`
	} `json:"ping"`
	Download struct {
		Bandwidth int64 `json:"bandwidth"`
	} `json:"download"`
	Upload struct {
		Bandwidth int64 `json:"bandwidth"`
	} `json:"upload"`
	Server struct {
		Name     string `json:"name"`
		Location string `json:"location"`
	} `json:"server"`
	ISP string `json:"isp"`
}

func parseSpeedtestOutput(data []byte) (SpeedtestResult, error) {
	var raw speedtestJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return SpeedtestResult{}, fmt.Errorf("parse speedtest json: %w", err)
	}

	return SpeedtestResult{
		DownloadMbps:   float64(raw.Download.Bandwidth) * 8 / 1_000_000,
		UploadMbps:     float64(raw.Upload.Bandwidth) * 8 / 1_000_000,
		LatencyMs:      raw.Ping.Latency,
		ServerName:     raw.Server.Name,
		ServerLocation: raw.Server.Location,
		ISP:            raw.ISP,
	}, nil
}

type SpeedtestCollector struct {
	serverID string
}

func NewSpeedtestCollector(serverID string) *SpeedtestCollector {
	return &SpeedtestCollector{serverID: serverID}
}

func (s *SpeedtestCollector) Collect(ctx context.Context) (SpeedtestResult, error) {
	slog.Info("running speed test...")
	args := []string{"--format=json", "--accept-license", "--accept-gdpr"}
	if s.serverID != "" {
		args = append(args, "--server-id="+s.serverID)
	}
	cmd := exec.CommandContext(ctx, "speedtest", args...)
	output, err := cmd.Output()
	if err != nil {
		return SpeedtestResult{}, fmt.Errorf("run speedtest: %w", err)
	}

	result, err := parseSpeedtestOutput(output)
	if err != nil {
		return SpeedtestResult{}, err
	}

	slog.Info("speed test complete",
		"download_mbps", fmt.Sprintf("%.1f", result.DownloadMbps),
		"upload_mbps", fmt.Sprintf("%.1f", result.UploadMbps),
		"latency_ms", fmt.Sprintf("%.1f", result.LatencyMs),
		"server", result.ServerName,
		"isp", result.ISP,
	)
	return result, nil
}
