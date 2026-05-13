package postrender

import "testing"

func TestHelpArgs(t *testing.T) {
	tests := []struct {
		binary string
		want   []string
	}{
		{"obscuro", []string{"version"}},
		{"helm", []string{"--help"}},
		{"", []string{"--help"}},
	}
	for _, tc := range tests {
		got := helpArgs(tc.binary)
		if len(got) != len(tc.want) {
			t.Fatalf("helpArgs(%q) length = %d, want %d", tc.binary, len(got), len(tc.want))
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("helpArgs(%q)[%d] = %q, want %q", tc.binary, i, got[i], tc.want[i])
			}
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   \n  ", ""},
		{"single line", "v1.2.3", "v1.2.3"},
		{"multi line", "v1.2.3\nmore info", "v1.2.3"},
		{"trim leading whitespace", "  v1.2.3\nx", "v1.2.3"},
		{"trailing newline", "v1.2.3\n", "v1.2.3"},
	}
	for _, tc := range tests {
		got := parseVersion(tc.in)
		if got != tc.want {
			t.Fatalf("%s: parseVersion(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}
