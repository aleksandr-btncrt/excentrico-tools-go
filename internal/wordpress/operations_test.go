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

func TestCreateWordPressSlug(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Hello World", "hello-world"},
		{"El Niño: Café & Señal!", "el-nino-cafe-senal"},
		{"  --Already-Slug--  ", "already-slug"},
		{"ÀÉÎÕÜ ç", "aeiou-c"},
		{"2026 - Sección Oficial - Película", "2026-seccion-oficial-pelicula"},
		{"", ""},
		{"!!!", ""},
	}
	for _, tt := range tests {
		if got := CreateWordPressSlug(tt.in); got != tt.want {
			t.Errorf("CreateWordPressSlug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	long := CreateWordPressSlug("a very long film title that goes on and on well beyond fifty characters")
	if len(long) != 50 {
		t.Errorf("long slug length = %d, want 50 (%q)", len(long), long)
	}
}

func TestIsPathUnderDirectorDir(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"films/x/Dir/jane_web.jpg", true},
		{"films/x/dir/jane_web.jpg", true},
		{"films/x/Stills/a_web.jpg", false},
		{"films/x/Director/a_web.jpg", false},
	}
	for _, tt := range tests {
		if got := isPathUnderDirectorDir(tt.in); got != tt.want {
			t.Errorf("isPathUnderDirectorDir(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
