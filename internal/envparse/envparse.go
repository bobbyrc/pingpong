package envparse

import "strings"

// CleanKey strips the "export " prefix from a key if present.
// "export FOO" -> "FOO". Keys like "EXPORTED_VAR" are untouched.
func CleanKey(key string) string {
	after, found := strings.CutPrefix(key, "export")
	if !found {
		return key
	}
	trimmed := strings.TrimLeft(after, " \t")
	// If nothing was trimmed, "export" is part of the key name (e.g. "EXPORTED_VAR")
	// or the key is literally "export" with nothing after it — return as-is.
	if after == trimmed {
		return key
	}
	// Guard against malformed lines like "export =value" producing an empty key.
	if trimmed == "" {
		return key
	}
	return trimmed
}

// CleanValue strips matched surrounding single or double quotes from a value.
// Mismatched or absent quotes are left untouched.
func CleanValue(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') ||
		(value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}
