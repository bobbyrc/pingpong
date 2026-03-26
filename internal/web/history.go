package web

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

// historyPoint is a single timestamped metric value.
type historyPoint struct {
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

// record inserts a single metric data point.
func (s *HistoryStore) record(metric, target string, value float64) error {
	_, err := s.db.Exec(
		"INSERT INTO metric_history (metric, target, value) VALUES (?, ?, ?)",
		metric, target, value,
	)
	return err
}

// prune deletes all but the newest `keep` rows for a given metric+target.
func (s *HistoryStore) prune(metric, target string, keep int) error {
	_, err := s.db.Exec(`DELETE FROM metric_history
		WHERE metric = ? AND target = ? AND id NOT IN (
			SELECT id FROM metric_history
			WHERE metric = ? AND target = ?
			ORDER BY id DESC LIMIT ?
		)`, metric, target, metric, target, keep)
	return err
}

// historyRow is a helper for scanning full history rows in a single query.
type historyRow struct {
	Metric string  `db:"metric"`
	Target string  `db:"target"`
	Time   string  `db:"recorded_at"`
	Value  float64 `db:"value"`
}

// loadAll returns all stored history grouped by metric then target.
// Result: map[metric]map[target][]historyPoint
func (s *HistoryStore) loadAll(limit int) (map[string]map[string][]historyPoint, error) {
	var rows []historyRow

	if limit > 0 {
		// Use a window function to enforce a per-series limit in SQL,
		// avoiding loading the entire table into memory.
		if err := s.db.Select(&rows, `
			SELECT metric, target, recorded_at, value
			FROM (
				SELECT
					metric, target, recorded_at, value, id,
					ROW_NUMBER() OVER (
						PARTITION BY metric, target
						ORDER BY id DESC
					) AS rn
				FROM metric_history
			)
			WHERE rn <= ?
			ORDER BY metric, target, id DESC
		`, limit); err != nil {
			return nil, err
		}
	} else {
		if err := s.db.Select(&rows, `SELECT metric, target, recorded_at, value
			FROM metric_history
			ORDER BY metric, target, id DESC`); err != nil {
			return nil, err
		}
	}

	result := make(map[string]map[string][]historyPoint)

	for _, r := range rows {
		if result[r.Metric] == nil {
			result[r.Metric] = make(map[string][]historyPoint)
		}
		result[r.Metric][r.Target] = append(result[r.Metric][r.Target], historyPoint{
			Time:  r.Time,
			Value: r.Value,
		})
	}

	// Reverse each series to oldest-first order, matching Load().
	for _, targets := range result {
		for target, points := range targets {
			for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
				points[i], points[j] = points[j], points[i]
			}
			targets[target] = points
		}
	}

	return result, nil
}
