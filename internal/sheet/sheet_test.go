package sheet

import (
	"reflect"
	"strings"
	"testing"
)

// TestDirectors verifies director extraction from the raw sheet row (ported
// from wordpress.TestParseDirectorNames). It guards against the regression
// where duplicate director images were uploaded for every film because the
// lookup used keys that did not match the sheet columns ("Direccion",
// "MultiDir"), so director image de-duplication never triggered.
func TestDirectors(t *testing.T) {
	headers := []string{ColDireccion, ColMultiDir}
	tests := []struct {
		name string
		row  []any
		hdrs []string
		want []string
	}{
		{"missing DIRECCIÓN column", []any{"SI"}, []string{ColMultiDir}, nil},
		{"empty DIRECCIÓN value", []any{"", ""}, headers, nil},
		{"single director, no Multi Dir flag", []any{"Jane Doe"}, headers, []string{"Jane Doe"}},
		{"single director, Multi Dir NO", []any{"Jane Doe", "NO"}, headers, []string{"Jane Doe"}},
		{"Multi Dir NO keeps separators", []any{"Jane Doe y John Smith", "NO"}, headers, []string{"Jane Doe y John Smith"}},
		{"comma", []any{"Jane Doe, John Smith", "SI"}, headers, []string{"Jane Doe", "John Smith"}},
		{"'y' with lower-case flag", []any{"Jane Doe y John Smith", " si "}, headers, []string{"Jane Doe", "John Smith"}},
		{"'+' and '&'", []any{"Jane Doe + John Smith & Alex Ray", "SI"}, headers, []string{"Jane Doe", "John Smith", "Alex Ray"}},
		{"names containing y are kept", []any{"Maya Deren y Yoko Ono", "SI"}, headers, []string{"Maya Deren", "Yoko Ono"}},
		{"legacy/incorrect keys are ignored", []any{"Jane Doe", "SI"}, []string{"Direccion", "MultiDir"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RowToFilm(tt.hdrs, tt.row).Directors()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Directors() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSplitDirectors(t *testing.T) {
	got := SplitDirectors(" A, B y C & D + E ")
	want := []string{"A", "B", "C", "D", "E"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SplitDirectors = %#v, want %#v", got, want)
	}
	if got := SplitDirectors(""); got != nil {
		t.Errorf("SplitDirectors(\"\") = %#v, want nil", got)
	}
}

func TestRowToFilm(t *testing.T) {
	headers := Headers([]any{ColTituloOriginal, nil, " " + ColEdicion + " ", ColAno, "Extra", ColEnlaces})
	f := RowToFilm(headers, []any{"Meshes", "ignored", "Excéntrico 2026", float64(1943), "note"})

	if f.TituloOriginal != "Meshes" || f.Edicion != "Excéntrico 2026" {
		t.Errorf("columns not aligned: %+v", f)
	}
	if f.Ano != "1943" {
		t.Errorf("numeric cell = %q, want 1943", f.Ano)
	}
	if f.Enlaces != "" {
		t.Errorf("missing trailing cell = %q, want empty", f.Enlaces)
	}
	if !reflect.DeepEqual(f.AdditionalFields, map[string]string{"Extra": "note"}) {
		t.Errorf("AdditionalFields = %#v", f.AdditionalFields)
	}
}

func TestFilmsAndTitle(t *testing.T) {
	if Films([][]any{{ColTituloOriginal}}) != nil {
		t.Error("header-only sheet should have no films")
	}
	films := Films([][]any{
		{ColTituloOriginal, ColName},
		{"Meshes", "x"},
		{"", "Fallback"},
		{"  ", ""},
	})
	var titles []string
	for _, f := range films {
		titles = append(titles, f.Title())
	}
	if got := strings.Join(titles, "|"); got != "Meshes|Fallback|"+UnnamedFilm {
		t.Errorf("titles = %q", got)
	}
	if films[1].HasTitle() || !films[0].HasTitle() {
		t.Error("HasTitle should reflect TÍTULO ORIGINAL only")
	}
}

func TestOptionsSelect(t *testing.T) {
	films := Films([][]any{
		{ColTituloOriginal, ColEdicion},
		{"Meshes of the Afternoon", "Excéntrico 2026"},
		{"La Jetée", " excéntrico 2026 "},
		{"Old Film", "Excéntrico 2025"},
		{"Other Fest", "Otro 2026"},
	})
	titles := func(fs []*Film) string {
		var ts []string
		for _, f := range fs {
			ts = append(ts, f.Title())
		}
		return strings.Join(ts, "|")
	}
	o := DefaultOptions()
	tests := []struct {
		opts             Options
		year, film, want string
	}{
		{o, "", "", "Meshes of the Afternoon|La Jetée|Old Film|Other Fest"},
		{o, "2026", "", "Meshes of the Afternoon|La Jetée"},
		{o, "2026", "la jetée", "La Jetée"},
		{o, "2026", "Old Film", ""},
		{o, "", "  old film ", "Old Film"},
		{Options{Tab: "X", EditionPrefix: "Otro "}, "2026", "", "Other Fest"},
	}
	for _, tt := range tests {
		if got := titles(tt.opts.Select(films, tt.year, tt.film)); got != tt.want {
			t.Errorf("Select(prefix=%q, year=%q, film=%q) = %q, want %q", tt.opts.EditionPrefix, tt.year, tt.film, got, tt.want)
		}
	}
	if o.DataRange() != "TODO!A:ZZ" || o.HeaderRange() != "TODO!1:1" {
		t.Errorf("ranges = %q, %q", o.DataRange(), o.HeaderRange())
	}
}

// TestCellString was ported from utils.TestCellString.
func TestCellString(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"", ""},
		{"Excéntrico 2026", "Excéntrico 2026"},
		{float64(2026), "2026"},
		{1.5, "1.5"},
		{42, "42"},
		{true, "true"},
	}
	for _, tt := range tests {
		if got := cellString(tt.in); got != tt.want {
			t.Errorf("cellString(%#v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
