package alerter

import (
	"path/filepath"
	"testing"
	"time"
)

func TestQueueEnqueueAndPending(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	err = q.Enqueue("downtime", "Connection Down", "Internet has been down for 2 minutes")
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
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	q.Enqueue("latency", "High Latency", "Ping is 200ms")

	pending, _ := q.Pending()
	err = q.MarkSent(pending[0].ID)
	if err != nil {
		t.Fatalf("failed to mark sent: %v", err)
	}

	pending, _ = q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after mark sent, got %d", len(pending))
	}
}

func TestQueueIncrementRetry(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	q.Enqueue("packet_loss", "Packet Loss", "50% packet loss")
	pending, _ := q.Pending()

	err = q.IncrementRetry(pending[0].ID)
	if err != nil {
		t.Fatalf("failed to increment retry: %v", err)
	}

	pending, _ = q.Pending()
	if pending[0].RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", pending[0].RetryCount)
	}
}

func TestQueueMarkFailedPermanent(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	q.Enqueue("speed", "Slow Speed", "Download is 5 Mbps")
	pending, _ := q.Pending()

	err = q.MarkFailedPermanent(pending[0].ID)
	if err != nil {
		t.Fatalf("failed to mark failed permanent: %v", err)
	}

	pending, _ = q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after permanent fail, got %d", len(pending))
	}
}

func TestQueueLastSentTime(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	_, found, err := q.LastSentTime("downtime")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected no last sent time for new queue")
	}

	q.Enqueue("downtime", "Down", "down")
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

func TestQueuePersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	q1, _ := NewQueue(dbPath)
	q1.Enqueue("downtime", "Down", "Internet down")
	q1.Close()

	q2, _ := NewQueue(dbPath)
	defer q2.Close()
	pending, _ := q2.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert after reopen, got %d", len(pending))
	}
}
