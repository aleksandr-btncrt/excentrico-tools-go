package wordpress

import (
	"reflect"
	"testing"
)

// TestParseDirectorNames verifies that director names are correctly extracted
// from the raw Google Sheet row (map[string]any), using the actual column
// names ("DIRECCIÓN" and "Multi Dir") rather than the mapped FilmData struct
// field names. This guards against a regression where duplicate director
// images were uploaded for every film because the lookup keys did not match
// the raw sheet columns, causing director image deduplication to never
// trigger (see issue: "Multiples Dir images due to multiples movies").
func TestParseDirectorNames(t *testing.T) {
	tests := []struct {
		name     string
		filmData map[string]any
		want     []string
	}{
		{
			name:     "nil film data",
			filmData: nil,
			want:     nil,
		},
		{
			name:     "missing DIRECCIÓN key",
			filmData: map[string]any{"Multi Dir": "SI"},
			want:     nil,
		},
		{
			name:     "empty DIRECCIÓN value",
			filmData: map[string]any{"DIRECCIÓN": ""},
			want:     nil,
		},
		{
			name: "single director, no Multi Dir flag",
			filmData: map[string]any{
				"DIRECCIÓN": "Jane Doe",
			},
			want: []string{"Jane Doe"},
		},
		{
			name: "single director, Multi Dir set to NO",
			filmData: map[string]any{
				"DIRECCIÓN": "Jane Doe",
				"Multi Dir": "NO",
			},
			want: []string{"Jane Doe"},
		},
		{
			name: "multiple directors separated by comma",
			filmData: map[string]any{
				"DIRECCIÓN": "Jane Doe, John Smith",
				"Multi Dir": "SI",
			},
			want: []string{"Jane Doe", "John Smith"},
		},
		{
			name: "multiple directors separated by 'y'",
			filmData: map[string]any{
				"DIRECCIÓN": "Jane Doe y John Smith",
				"Multi Dir": "si",
			},
			want: []string{"Jane Doe", "John Smith"},
		},
		{
			name: "multiple directors separated by '+' and '&'",
			filmData: map[string]any{
				"DIRECCIÓN": "Jane Doe + John Smith & Alex Ray",
				"Multi Dir": "SI",
			},
			want: []string{"Jane Doe", "John Smith", "Alex Ray"},
		},
		{
			name: "legacy/incorrect keys are ignored",
			filmData: map[string]any{
				"Direccion": "Jane Doe",
				"MultiDir":  "SI",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDirectorNames(tt.filmData)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDirectorNames(%v) = %v, want %v", tt.filmData, got, tt.want)
			}
		})
	}
}

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
