package web

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ReadEnvFile reads a .env file and returns a map of key=value pairs.
// Comments (lines starting with #) and blank lines are skipped.
func ReadEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open env file: %w", err)
	}
	defer f.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		env[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan env file: %w", err)
	}
	return env, nil
}

// WriteEnvFile updates a .env file with the provided key=value pairs.
// Existing keys are updated in place, preserving comments and blank lines.
// New keys (those not already present in the file) are appended at the end.
func WriteEnvFile(path string, updates map[string]string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open env file: %w", err)
	}
	defer f.Close()

	var lines []string
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Preserve comments and blank lines as-is.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			lines = append(lines, line)
			continue
		}

		key, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			lines = append(lines, line)
			continue
		}

		if newVal, found := updates[key]; found {
			lines = append(lines, key+"="+newVal)
			seen[key] = true
		} else {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan env file: %w", err)
	}

	// Append any new keys not already in the file.
	var newKeys []string
	for key := range updates {
		if !seen[key] {
			newKeys = append(newKeys, key)
		}
	}
	sort.Strings(newKeys)
	for _, key := range newKeys {
		lines = append(lines, key+"="+updates[key])
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}
	return nil
}
