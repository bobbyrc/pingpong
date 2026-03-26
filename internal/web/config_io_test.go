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

	env, err := readEnvFile(path)
	if err != nil {
		t.Fatalf("readEnvFile: %v", err)
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

	env, err := readEnvFile(path)
	if err != nil {
		t.Fatalf("readEnvFile: %v", err)
	}

	if env["KEY"] != "val=ue" {
		t.Errorf("KEY = %q, want %q", env["KEY"], "val=ue")
	}
	if env["EMPTY"] != "" {
		t.Errorf("EMPTY = %q, want %q", env["EMPTY"], "")
	}
}

func TestReadEnvFile_StripsQuotesAndExportPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	content := "DOUBLE=\"hello\"\nSINGLE='world'\nexport EXPORTED=val\nexport BOTH=\"quoted\"\nPLAIN=unchanged\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := readEnvFile(path)
	if err != nil {
		t.Fatalf("readEnvFile: %v", err)
	}

	tests := []struct {
		key, want string
	}{
		{"DOUBLE", "hello"},
		{"SINGLE", "world"},
		{"EXPORTED", "val"},
		{"BOTH", "quoted"},
		{"PLAIN", "unchanged"},
	}
	for _, tt := range tests {
		if env[tt.key] != tt.want {
			t.Errorf("%s = %q, want %q", tt.key, env[tt.key], tt.want)
		}
	}
}

func TestReadEnvFile_NotFound(t *testing.T) {
	env, err := readEnvFile(filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(env) != 0 {
		t.Fatalf("expected empty map, got %v", env)
	}
}

func TestReadEnvFile_ParentDirMissing(t *testing.T) {
	_, err := readEnvFile(filepath.Join(t.TempDir(), "no-such-dir", ".env"))
	if err == nil {
		t.Fatal("expected error when parent directory does not exist")
	}
}

func TestWriteEnvFile_ParentDirMissing(t *testing.T) {
	err := writeEnvFile(filepath.Join(t.TempDir(), "no-such-dir", ".env"), map[string]string{"A": "1"})
	if err == nil {
		t.Fatal("expected error when parent directory does not exist")
	}
}

func TestWriteEnvFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	updates := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}
	if err := writeEnvFile(path, updates); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := string(data)
	if !strings.Contains(result, "FOO=bar") {
		t.Error("FOO not written")
	}
	if !strings.Contains(result, "BAZ=qux") {
		t.Error("BAZ not written")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
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
	if err := writeEnvFile(path, updates); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
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
	if err := writeEnvFile(path, updates); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
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
	if err := writeEnvFile(path, updates); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
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

func TestWriteEnvFile_UpdatesExportPrefixedKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	original := "# config\nexport FOO=old\nBAR=keep\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	updates := map[string]string{"FOO": "new"}
	if err := writeEnvFile(path, updates); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := string(data)

	if !strings.Contains(result, "FOO=new") {
		t.Error("FOO not updated")
	}
	if strings.Contains(result, "export FOO=old") {
		t.Error("old export FOO line still present")
	}
	if strings.Count(result, "FOO=") != 1 {
		t.Errorf("expected exactly one FOO= line, got:\n%s", result)
	}
	if !strings.Contains(result, "BAR=keep") {
		t.Error("BAR not preserved")
	}
}
