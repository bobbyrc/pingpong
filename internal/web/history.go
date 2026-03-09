package web

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

// HistoryPoint is a single timestamped metric value.
type HistoryPoint struct {
	Time  string  `db:"recorded_at" json:"t"`
	Value float64 `db:"value"       json:"v"`
}

// HistoryStore persists sparkline metric history in SQLite.
type HistoryStore struct {
	db *sqlx.DB
}

// NewHistoryStore creates the metric_history table if it doesn't exist.
func NewHistoryStore(db *sqlx.DB) (*HistoryStore, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS metric_history (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		recorded_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		metric      TEXT NOT NULL,
		target      TEXT NOT NULL DEFAULT '',
		value       REAL NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create metric_history table: %w", err)
	}

	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_metric_history_lookup
		ON metric_history (metric, target, recorded_at DESC)`); err != nil {
		return nil, fmt.Errorf("create metric_history index: %w", err)
	}

	return &HistoryStore{db: db}, nil
}

// Record inserts a single metric data point.
func (s *HistoryStore) Record(metric, target string, value float64) error {
	_, err := s.db.Exec(
		"INSERT INTO metric_history (metric, target, value) VALUES (?, ?, ?)",
		metric, target, value,
	)
	return err
}

// Prune deletes all but the newest `keep` rows for a given metric+target.
func (s *HistoryStore) Prune(metric, target string, keep int) error {
	_, err := s.db.Exec(`DELETE FROM metric_history
		WHERE metric = ? AND target = ? AND id NOT IN (
			SELECT id FROM metric_history
			WHERE metric = ? AND target = ?
			ORDER BY id DESC LIMIT ?
		)`, metric, target, metric, target, keep)
	return err
}

// Load returns the last `limit` data points for a metric+target, oldest first.
func (s *HistoryStore) Load(metric, target string, limit int) ([]HistoryPoint, error) {
	var points []HistoryPoint
	err := s.db.Select(&points, `SELECT recorded_at, value FROM metric_history
		WHERE metric = ? AND target = ?
		ORDER BY id DESC LIMIT ?`, metric, target, limit)
	if err != nil {
		return nil, err
	}
	// Reverse to oldest-first order
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
	return points, nil
}

// distinctSeries is a helper struct for scanning distinct metric+target pairs.
type distinctSeries struct {
	Metric string `db:"metric"`
	Target string `db:"target"`
}

// LoadAll returns all stored history grouped by metric then target.
// Result: map[metric]map[target][]HistoryPoint
func (s *HistoryStore) LoadAll(limit int) (map[string]map[string][]HistoryPoint, error) {
	var series []distinctSeries
	if err := s.db.Select(&series,
		"SELECT DISTINCT metric, target FROM metric_history"); err != nil {
		return nil, err
	}

	result := make(map[string]map[string][]HistoryPoint)
	for _, ds := range series {
		points, err := s.Load(ds.Metric, ds.Target, limit)
		if err != nil {
			return nil, err
		}
		if result[ds.Metric] == nil {
			result[ds.Metric] = make(map[string][]HistoryPoint)
		}
		result[ds.Metric][ds.Target] = points
	}
	return result, nil
}
