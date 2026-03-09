package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	content := "# comment\nFOO=bar\nBAZ=qux\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}

	if len(env) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(env))
	}
	if env["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", env["FOO"], "bar")
	}
	if env["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want %q", env["BAZ"], "qux")
	}

	// Verify comments are not in the map.
	for k := range env {
		if strings.HasPrefix(k, "#") {
			t.Errorf("unexpected comment key: %q", k)
		}
	}
}

func TestReadEnvFile_BlankLinesAndValuesWithEquals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	content := "# section\n\nKEY=val=ue\nEMPTY=\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}

	if env["KEY"] != "val=ue" {
		t.Errorf("KEY = %q, want %q", env["KEY"], "val=ue")
	}
	if env["EMPTY"] != "" {
		t.Errorf("EMPTY = %q, want %q", env["EMPTY"], "")
	}
}

func TestReadEnvFile_NotFound(t *testing.T) {
	_, err := ReadEnvFile("/nonexistent/.env")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestWriteEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	original := "# PingPong Configuration\n\nFOO=old\n# another comment\nBAR=keep\nBAZ=old\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	updates := map[string]string{
		"FOO": "new",
		"BAZ": "updated",
	}
	if err := WriteEnvFile(path, updates); err != nil {
		t.Fatalf("WriteEnvFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := string(data)

	// Comments are preserved.
	if !strings.Contains(result, "# PingPong Configuration") {
		t.Error("first comment not preserved")
	}
	if !strings.Contains(result, "# another comment") {
		t.Error("second comment not preserved")
	}

	// Updated values are correct.
	if !strings.Contains(result, "FOO=new") {
		t.Error("FOO not updated")
	}
	if !strings.Contains(result, "BAZ=updated") {
		t.Error("BAZ not updated")
	}

	// Untouched values are preserved.
	if !strings.Contains(result, "BAR=keep") {
		t.Error("BAR not preserved")
	}

	// Old values are gone.
	if strings.Contains(result, "FOO=old") {
		t.Error("FOO still has old value")
	}
	if strings.Contains(result, "BAZ=old") {
		t.Error("BAZ still has old value")
	}
}

func TestWriteEnvFile_AppendsNewKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	original := "# config\nEXISTING=yes\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	updates := map[string]string{
		"NEWKEY": "newval",
	}
	if err := WriteEnvFile(path, updates); err != nil {
		t.Fatalf("WriteEnvFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := string(data)

	if !strings.Contains(result, "EXISTING=yes") {
		t.Error("existing key not preserved")
	}
	if !strings.Contains(result, "NEWKEY=newval") {
		t.Error("new key not appended")
	}
	if !strings.Contains(result, "# config") {
		t.Error("comment not preserved")
	}
}

func TestWriteEnvFile_PreservesStructure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	// Mimics the real .env.example structure with sections.
	original := "# === Section A ===\nA=1\n\n# === Section B ===\nB=2\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	updates := map[string]string{"A": "10"}
	if err := WriteEnvFile(path, updates); err != nil {
		t.Fatalf("WriteEnvFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	expected := []string{
		"# === Section A ===",
		"A=10",
		"",
		"# === Section B ===",
		"B=2",
	}
	if len(lines) != len(expected) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(expected), string(data))
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}
}
