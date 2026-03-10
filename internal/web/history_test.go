package web

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestHistoryStore_RecordAndLoad(t *testing.T) {
	db := openTestDB(t)
	store, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}

	// Record some values
	for i := 0; i < 5; i++ {
		if err := store.Record("ping_latency", "1.1.1.1", float64(10+i)); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	points, err := store.Load("ping_latency", "1.1.1.1", 60)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(points) != 5 {
		t.Fatalf("expected 5 points, got %d", len(points))
	}
	// Verify oldest-first ordering
	if points[0].Value != 10 {
		t.Errorf("first point = %v, want 10", points[0].Value)
	}
	if points[4].Value != 14 {
		t.Errorf("last point = %v, want 14", points[4].Value)
	}
	// Verify timestamps are populated
	if points[0].Time == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestHistoryStore_Prune(t *testing.T) {
	db := openTestDB(t)
	store, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}

	// Record 10 values
	for i := 0; i < 10; i++ {
		if err := store.Record("ping_latency", "8.8.8.8", float64(i)); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	// Prune to keep 3
	if err := store.Prune("ping_latency", "8.8.8.8", 3); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	points, err := store.Load("ping_latency", "8.8.8.8", 60)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 points after prune, got %d", len(points))
	}
	// Should keep the newest 3: values 7, 8, 9
	if points[0].Value != 7 {
		t.Errorf("first point after prune = %v, want 7", points[0].Value)
	}
	if points[2].Value != 9 {
		t.Errorf("last point after prune = %v, want 9", points[2].Value)
	}
}

func TestHistoryStore_MultipleSeries(t *testing.T) {
	db := openTestDB(t)
	store, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}

	store.Record("ping_latency", "1.1.1.1", 10)
	store.Record("ping_latency", "8.8.8.8", 20)
	store.Record("download_speed", "", 100)

	p1, _ := store.Load("ping_latency", "1.1.1.1", 60)
	p2, _ := store.Load("ping_latency", "8.8.8.8", 60)
	p3, _ := store.Load("download_speed", "", 60)

	if len(p1) != 1 || p1[0].Value != 10 {
		t.Errorf("1.1.1.1: got %v", p1)
	}
	if len(p2) != 1 || p2[0].Value != 20 {
		t.Errorf("8.8.8.8: got %v", p2)
	}
	if len(p3) != 1 || p3[0].Value != 100 {
		t.Errorf("download: got %v", p3)
	}
}

func TestHistoryStore_LoadEmpty(t *testing.T) {
	db := openTestDB(t)
	store, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}

	points, err := store.Load("nonexistent", "target", 60)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected 0 points, got %d", len(points))
	}
}

func TestHistoryStore_LoadAll(t *testing.T) {
	db := openTestDB(t)
	store, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}

	store.Record("ping_latency", "1.1.1.1", 10)
	store.Record("ping_latency", "8.8.8.8", 20)
	store.Record("download_speed", "", 100)
	store.Record("upload_speed", "", 50)

	all, err := store.LoadAll(60)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// Verify structure
	pingData, ok := all["ping_latency"]
	if !ok {
		t.Fatal("missing ping_latency key")
	}
	if len(pingData["1.1.1.1"]) != 1 {
		t.Errorf("expected 1 point for 1.1.1.1, got %d", len(pingData["1.1.1.1"]))
	}
	if len(pingData["8.8.8.8"]) != 1 {
		t.Errorf("expected 1 point for 8.8.8.8, got %d", len(pingData["8.8.8.8"]))
	}

	dlData, ok := all["download_speed"]
	if !ok {
		t.Fatal("missing download_speed key")
	}
	if len(dlData[""]) != 1 {
		t.Errorf("expected 1 point for download_speed, got %d", len(dlData[""]))
	}
}

func TestHistoryStore_LoadAllWithLimit(t *testing.T) {
	db := openTestDB(t)
	store, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}

	// Record 10 points for one series
	for i := 0; i < 10; i++ {
		if err := store.Record("ping_latency", "1.1.1.1", float64(i)); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	all, err := store.LoadAll(3)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	pingData, ok := all["ping_latency"]
	if !ok {
		t.Fatal("missing ping_latency key")
	}
	points := pingData["1.1.1.1"]
	if len(points) != 3 {
		t.Fatalf("expected 3 points with limit=3, got %d", len(points))
	}

	// Should be the newest 3 (values 7, 8, 9) in oldest-first order
	if points[0].Value != 7 {
		t.Errorf("first point = %v, want 7", points[0].Value)
	}
	if points[2].Value != 9 {
		t.Errorf("last point = %v, want 9", points[2].Value)
	}
}

func TestHistoryStore_PruneNonExistent(t *testing.T) {
	db := openTestDB(t)
	store, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}

	// Prune a metric/target that has no data; should not error
	if err := store.Prune("nonexistent_metric", "no_target", 3); err != nil {
		t.Fatalf("Prune on non-existent data returned error: %v", err)
	}
}

func TestHistoryStore_NewHistoryStoreIdempotent(t *testing.T) {
	db := openTestDB(t)

	store1, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("first NewHistoryStore: %v", err)
	}

	store2, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("second NewHistoryStore: %v", err)
	}

	// Verify both stores work by writing with one and reading with the other
	if err := store1.Record("test_metric", "target", 42.0); err != nil {
		t.Fatalf("Record via store1: %v", err)
	}

	points, err := store2.Load("test_metric", "target", 60)
	if err != nil {
		t.Fatalf("Load via store2: %v", err)
	}
	if len(points) != 1 || points[0].Value != 42.0 {
		t.Errorf("expected 1 point with value 42.0, got %v", points)
	}
}
