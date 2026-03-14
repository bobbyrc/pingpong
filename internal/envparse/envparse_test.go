package envparse

import "testing"

func TestCleanValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// No quotes — pass through unchanged
		{name: "plain value", input: "hello", want: "hello"},
		{name: "empty string", input: "", want: ""},

		// Double quotes
		{name: "double quoted", input: `"hello"`, want: "hello"},
		{name: "double quoted empty", input: `""`, want: ""},
		{name: "double quoted with spaces", input: `"hello world"`, want: "hello world"},

		// Single quotes
		{name: "single quoted", input: "'hello'", want: "hello"},
		{name: "single quoted empty", input: "''", want: ""},

		// Mismatched quotes — leave untouched
		{name: "double open single close", input: `"hello'`, want: `"hello'`},
		{name: "single open double close", input: `'hello"`, want: `'hello"`},

		// Only opening quote — not a matched pair
		{name: "single double quote", input: `"`, want: `"`},
		{name: "single single quote", input: "'", want: "'"},

		// Internal quotes — only outer pair is stripped
		{name: "internal double quotes", input: `"he said \"hi\""`, want: `he said \"hi\"`},
		{name: "single quotes inside double", input: `"it's fine"`, want: "it's fine"},
		{name: "double quotes inside single", input: `'say "hi"'`, want: `say "hi"`},

		// Quotes not at both ends — leave untouched
		{name: "leading double quote only", input: `"hello`, want: `"hello`},
		{name: "trailing double quote only", input: `hello"`, want: `hello"`},

		// Quoted whitespace
		{name: "quoted whitespace", input: `" "`, want: " "},

		// Values with special characters
		{name: "URL with quotes", input: `"discord://webhook/123"`, want: "discord://webhook/123"},
		{name: "URL without quotes", input: "discord://webhook/123", want: "discord://webhook/123"},
		{name: "value with equals", input: `"a=b=c"`, want: "a=b=c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanValue(tt.input)
			if got != tt.want {
				t.Errorf("CleanValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// No prefix — pass through
		{name: "plain key", input: "FOO", want: "FOO"},
		{name: "empty string", input: "", want: ""},

		// export prefix stripped
		{name: "export prefix", input: "export FOO", want: "FOO"},
		{name: "export prefix extra spaces", input: "export   FOO", want: "FOO"},
		{name: "export prefix with tab", input: "export\tFOO", want: "FOO"},

		// export as part of key name — not stripped
		{name: "exported not stripped", input: "EXPORTED_VAR", want: "EXPORTED_VAR"},
		{name: "my_export not stripped", input: "MY_EXPORT", want: "MY_EXPORT"},

		// Edge cases
		{name: "export alone", input: "export", want: "export"},
		{name: "export with trailing space only", input: "export ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanKey(tt.input)
			if got != tt.want {
				t.Errorf("CleanKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
