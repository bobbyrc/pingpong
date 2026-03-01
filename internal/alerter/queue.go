package alerter

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type Alert struct {
	ID         int64      `db:"id"`
	CreatedAt  time.Time  `db:"created_at"`
	SentAt     *time.Time `db:"sent_at"`
	Status     string     `db:"status"`
	AlertType  string     `db:"alert_type"`
	Title      string     `db:"title"`
	Body       string     `db:"body"`
	RetryCount int        `db:"retry_count"`
}

type Queue struct {
	db *sqlx.DB
}

func NewQueue(dbPath string) (*Queue, error) {
	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.MustExec("PRAGMA journal_mode=WAL")

	db.MustExec(`CREATE TABLE IF NOT EXISTS alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		sent_at TIMESTAMP,
		status TEXT NOT NULL DEFAULT 'pending',
		alert_type TEXT NOT NULL,
		title TEXT NOT NULL,
		body TEXT NOT NULL,
		retry_count INTEGER NOT NULL DEFAULT 0
	)`)

	return &Queue{db: db}, nil
}

func (q *Queue) Close() error {
	return q.db.Close()
}

func (q *Queue) Enqueue(alertType, title, body string) error {
	_, err := q.db.Exec(
		"INSERT INTO alerts (alert_type, title, body) VALUES (?, ?, ?)",
		alertType, title, body,
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

func (q *Queue) LastSentTime(alertType string) (time.Time, bool, error) {
	var sentAt *time.Time
	err := q.db.Get(&sentAt,
		"SELECT sent_at FROM alerts WHERE alert_type = ? AND status = 'sent' ORDER BY sent_at DESC LIMIT 1",
		alertType,
	)
	if err != nil || sentAt == nil {
		return time.Time{}, false, nil
	}
	return *sentAt, true, nil
}
