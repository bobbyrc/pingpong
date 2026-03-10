package alerter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

// openTestQueue creates an OpenDB + NewQueue pair for testing.
// The database is closed automatically when the test finishes.
func openTestQueue(t *testing.T) *Queue {
	t.Helper()
	dir := t.TempDir()
	db, err := OpenDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	q, err := NewQueue(db)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	return q
}

// openTestQueueAt opens a database at a specific path for persistence tests.
func openTestQueueAt(t *testing.T, dbPath string) (*Queue, *sqlx.DB) {
	t.Helper()
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	q, err := NewQueue(db)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	return q, db
}

func TestQueueEnqueueAndPending(t *testing.T) {
	q := openTestQueue(t)

	err := q.Enqueue("downtime", "downtime", "Connection Down", "Internet has been down for 2 minutes")
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	pending, err := q.Pending()
	if err != nil {
		t.Fatalf("failed to get pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert, got %d", len(pending))
	}
	if pending[0].AlertType != "downtime" {
		t.Fatalf("expected alert type downtime, got %s", pending[0].AlertType)
	}
	if pending[0].Title != "Connection Down" {
		t.Fatalf("expected title 'Connection Down', got %s", pending[0].Title)
	}
	if pending[0].Status != "pending" {
		t.Fatalf("expected status pending, got %s", pending[0].Status)
	}
}

func TestQueueMarkSent(t *testing.T) {
	q := openTestQueue(t)

	q.Enqueue("latency:1.1.1.1", "latency", "High Latency", "Ping is 200ms")

	pending, _ := q.Pending()
	err := q.MarkSent(pending[0].ID)
	if err != nil {
		t.Fatalf("failed to mark sent: %v", err)
	}

	pending, _ = q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after mark sent, got %d", len(pending))
	}
}

func TestQueueIncrementRetry(t *testing.T) {
	q := openTestQueue(t)

	q.Enqueue("packet_loss:1.1.1.1", "packet_loss", "Packet Loss", "50% packet loss")
	pending, _ := q.Pending()

	err := q.IncrementRetry(pending[0].ID)
	if err != nil {
		t.Fatalf("failed to increment retry: %v", err)
	}

	pending, _ = q.Pending()
	if pending[0].RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", pending[0].RetryCount)
	}
}

func TestQueueMarkFailedPermanent(t *testing.T) {
	q := openTestQueue(t)

	q.Enqueue("speed", "speed", "Slow Speed", "Download is 5 Mbps")
	pending, _ := q.Pending()

	err := q.MarkFailedPermanent(pending[0].ID)
	if err != nil {
		t.Fatalf("failed to mark failed permanent: %v", err)
	}

	pending, _ = q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after permanent fail, got %d", len(pending))
	}
}

func TestQueueLastSentTime(t *testing.T) {
	q := openTestQueue(t)

	_, found, err := q.LastSentTime("downtime")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected no last sent time for new queue")
	}

	q.Enqueue("downtime", "downtime", "Down", "down")
	pending, _ := q.Pending()
	q.MarkSent(pending[0].ID)

	lastSent, found, err := q.LastSentTime("downtime")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected to find last sent time")
	}
	if time.Since(lastSent) > 5*time.Second {
		t.Fatalf("last sent time should be recent, got %v", lastSent)
	}
}

func TestRecentAlerts(t *testing.T) {
	q := openTestQueue(t)

	// Enqueue 3 alerts
	q.Enqueue("key1", "latency", "Alert 1", "Body 1")
	q.Enqueue("key2", "speed", "Alert 2", "Body 2")
	q.Enqueue("key3", "downtime", "Alert 3", "Body 3")

	// Mark one as sent
	q.MarkSent(1)

	// Page 1: limit 2, offset 0
	alerts, total, err := q.RecentAlerts(2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(alerts))
	}
	// Most recent first
	if alerts[0].Title != "Alert 3" {
		t.Errorf("expected Alert 3 first, got %s", alerts[0].Title)
	}

	// Page 2: limit 2, offset 2
	alerts, total, err = q.RecentAlerts(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
}

func TestRecentAlertsEmpty(t *testing.T) {
	q := openTestQueue(t)

	alerts, total, err := q.RecentAlerts(10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %d", total)
	}
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestOpenDBSetsPragmas(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Verify WAL mode
	var journalMode string
	if err := db.Get(&journalMode, "PRAGMA journal_mode"); err != nil {
		t.Fatalf("get journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected journal_mode=wal, got %q", journalMode)
	}

	// Verify busy_timeout
	var busyTimeout int
	if err := db.Get(&busyTimeout, "PRAGMA busy_timeout"); err != nil {
		t.Fatalf("get busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("expected busy_timeout=5000, got %d", busyTimeout)
	}
}

func TestDeleteAlert(t *testing.T) {
	q := openTestQueue(t)

	q.Enqueue("key1", "latency", "Alert 1", "Body 1")
	q.Enqueue("key2", "speed", "Alert 2", "Body 2")

	alerts, total, _ := q.RecentAlerts(10, 0)
	if total != 2 {
		t.Fatalf("expected 2 alerts, got %d", total)
	}

	err := q.DeleteAlert(alerts[0].ID)
	if err != nil {
		t.Fatalf("DeleteAlert: %v", err)
	}

	_, total, _ = q.RecentAlerts(10, 0)
	if total != 1 {
		t.Fatalf("expected 1 alert after delete, got %d", total)
	}
}

func TestDeleteAlertNonExistent(t *testing.T) {
	q := openTestQueue(t)

	err := q.DeleteAlert(9999)
	if err != nil {
		t.Fatalf("DeleteAlert non-existent should not error: %v", err)
	}
}

func TestDeleteAllAlerts(t *testing.T) {
	q := openTestQueue(t)

	q.Enqueue("key1", "latency", "Alert 1", "Body 1")
	q.Enqueue("key2", "speed", "Alert 2", "Body 2")
	q.Enqueue("key3", "downtime", "Alert 3", "Body 3")

	err := q.DeleteAllAlerts()
	if err != nil {
		t.Fatalf("DeleteAllAlerts: %v", err)
	}

	_, total, _ := q.RecentAlerts(10, 0)
	if total != 0 {
		t.Fatalf("expected 0 alerts after delete all, got %d", total)
	}
}

func TestDeleteAllAlertsEmpty(t *testing.T) {
	q := openTestQueue(t)

	err := q.DeleteAllAlerts()
	if err != nil {
		t.Fatalf("DeleteAllAlerts on empty should not error: %v", err)
	}
}

func TestAllCooldowns(t *testing.T) {
	q := openTestQueue(t)

	// Enqueue two alerts with different cooldown keys.
	q.Enqueue("latency:1.1.1.1", "latency", "High Latency", "Ping is 200ms")
	q.Enqueue("packet_loss:8.8.8.8", "packet_loss", "Packet Loss", "50% loss")

	pending, _ := q.Pending()
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d", len(pending))
	}

	// Mark only the first one as sent.
	if err := q.MarkSent(pending[0].ID); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}

	// AllCooldowns should return only the sent alert's cooldown key.
	cooldowns, err := q.AllCooldowns()
	if err != nil {
		t.Fatalf("AllCooldowns: %v", err)
	}
	if len(cooldowns) != 1 {
		t.Fatalf("expected 1 cooldown entry, got %d", len(cooldowns))
	}

	sentKey := pending[0].CooldownKey
	ts, ok := cooldowns[sentKey]
	if !ok {
		t.Fatalf("expected cooldown key %q in result, got keys: %v", sentKey, cooldowns)
	}
	if time.Since(ts) > 5*time.Second {
		t.Fatalf("cooldown timestamp should be recent, got %v", ts)
	}

	// The unsent alert's key should NOT appear.
	unsentKey := pending[1].CooldownKey
	if _, exists := cooldowns[unsentKey]; exists {
		t.Fatalf("did not expect cooldown key %q for unsent alert", unsentKey)
	}
}

func TestNewQueueIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// First call creates the table.
	q1, err := NewQueue(db)
	if err != nil {
		t.Fatalf("first NewQueue: %v", err)
	}

	// Enqueue something to prove the table works.
	if err := q1.Enqueue("test", "test", "Test", "body"); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Second call on the same DB should succeed (ALTER TABLE migration
	// handles "duplicate column" gracefully).
	q2, err := NewQueue(db)
	if err != nil {
		t.Fatalf("second NewQueue should succeed (idempotent), got: %v", err)
	}

	// Verify the data is still accessible via the second queue handle.
	pending, err := q2.Pending()
	if err != nil {
		t.Fatalf("Pending via q2: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert via second queue, got %d", len(pending))
	}
}

func TestQueuePersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	q1, db1 := openTestQueueAt(t, dbPath)
	q1.Enqueue("downtime", "downtime", "Down", "Internet down")
	db1.Close()

	q2, db2 := openTestQueueAt(t, dbPath)
	defer db2.Close()
	pending, _ := q2.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert after reopen, got %d", len(pending))
	}
}
