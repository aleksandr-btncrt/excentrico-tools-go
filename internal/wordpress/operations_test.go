package wordpress

import "testing"

func TestNormalizeDirectorCacheKey(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"trims and lowercases", "  Jane Doe  ", "jane doe"},
		{"already normalized", "jane doe", "jane doe"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeDirectorCacheKey(tt.in); got != tt.want {
				t.Errorf("normalizeDirectorCacheKey(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
