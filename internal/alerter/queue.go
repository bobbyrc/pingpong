package alerter

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type Alert struct {
	ID          int64      `db:"id"`
	CreatedAt   time.Time  `db:"created_at"`
	SentAt      *time.Time `db:"sent_at"`
	Status      string     `db:"status"`
	AlertType   string     `db:"alert_type"`
	CooldownKey string     `db:"cooldown_key"`
	Title       string     `db:"title"`
	Body        string     `db:"body"`
	RetryCount  int        `db:"retry_count"`
}

type Queue struct {
	db *sqlx.DB
}

func NewQueue(dbPath string) (*Queue, error) {
	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		sent_at TIMESTAMP,
		status TEXT NOT NULL DEFAULT 'pending',
		alert_type TEXT NOT NULL,
		cooldown_key TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL,
		body TEXT NOT NULL,
		retry_count INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		return nil, fmt.Errorf("create alerts table: %w", err)
	}

	// Migration: add cooldown_key column if missing (existing databases).
	db.Exec("ALTER TABLE alerts ADD COLUMN cooldown_key TEXT NOT NULL DEFAULT ''")

	return &Queue{db: db}, nil
}

func (q *Queue) Close() error {
	return q.db.Close()
}

func (q *Queue) Enqueue(cooldownKey, alertType, title, body string) error {
	_, err := q.db.Exec(
		"INSERT INTO alerts (cooldown_key, alert_type, title, body) VALUES (?, ?, ?, ?)",
		cooldownKey, alertType, title, body,
	)
	return err
}

func (q *Queue) Pending() ([]Alert, error) {
	var alerts []Alert
	err := q.db.Select(&alerts,
		"SELECT * FROM alerts WHERE status = 'pending' ORDER BY created_at ASC",
	)
	return alerts, err
}

func (q *Queue) MarkSent(id int64) error {
	_, err := q.db.Exec(
		"UPDATE alerts SET status = 'sent', sent_at = CURRENT_TIMESTAMP WHERE id = ?",
		id,
	)
	return err
}

func (q *Queue) IncrementRetry(id int64) error {
	_, err := q.db.Exec(
		"UPDATE alerts SET retry_count = retry_count + 1 WHERE id = ?",
		id,
	)
	return err
}

func (q *Queue) MarkFailedPermanent(id int64) error {
	_, err := q.db.Exec(
		"UPDATE alerts SET status = 'failed_permanent' WHERE id = ?",
		id,
	)
	return err
}

func (q *Queue) LastSentTime(cooldownKey string) (time.Time, bool, error) {
	var sentAt *time.Time
	err := q.db.Get(&sentAt,
		"SELECT sent_at FROM alerts WHERE cooldown_key = ? AND status = 'sent' ORDER BY sent_at DESC LIMIT 1",
		cooldownKey,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	if sentAt == nil {
		return time.Time{}, false, nil
	}
	return *sentAt, true, nil
}

type cooldownEntry struct {
	CooldownKey string     `db:"cooldown_key"`
	LastSent    *time.Time `db:"last_sent"`
}

// AllCooldowns returns the most recent sent_at for each distinct cooldown_key.
func (q *Queue) AllCooldowns() (map[string]time.Time, error) {
	var entries []cooldownEntry
	err := q.db.Select(&entries,
		"SELECT cooldown_key, MAX(sent_at) AS last_sent FROM alerts WHERE status = 'sent' AND cooldown_key != '' GROUP BY cooldown_key",
	)
	if err != nil {
		return nil, err
	}
	result := make(map[string]time.Time, len(entries))
	for _, e := range entries {
		if e.LastSent != nil {
			result[e.CooldownKey] = *e.LastSent
		}
	}
	return result, nil
}
