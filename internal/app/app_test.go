package app

import (
	"strings"
	"testing"

	"excentrico-tools-go/internal/models"
)

func TestFormatDirectors(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Maya Deren", "MAYA DEREN"},
		{"Ana López y Yolanda Pérez", "ANA LÓPEZ & YOLANDA PÉREZ"},
		{"A, B y C", "A & B & C"},
		{"A & B", "A & B"},
		{"A + B", "A & B"},
		{"  Solo  ", "SOLO"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := formatDirectors(tt.in); got != tt.want {
			t.Errorf("formatDirectors(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	got, err := parseDate("2026-01-23")
	if err != nil {
		t.Fatal(err)
	}
	if got != "viernes 23 de enero" {
		t.Errorf("parseDate = %q, want %q", got, "viernes 23 de enero")
	}

	for _, bad := range []string{"", "2026-01", "23/01/2026", "2026-13-01"} {
		if _, err := parseDate(bad); err == nil {
			t.Errorf("parseDate(%q) expected error", bad)
		}
	}
}

func TestCreateMetadata(t *testing.T) {
	meta := &models.Metadata{
		Cities: []string{"Valparaíso"},
		Dates:  [][]string{{"2026-01-23", "2026-01-30"}},
	}

	got, err := CreateMetadata("La Película", "Competencia", "Maya Deren y Yoko Ono", meta, "2026")
	if err != nil {
		t.Fatal(err)
	}
	want := "COMPETENCIA 2026 - La Película - MAYA DEREN & YOKO ONO - Programación Excéntrico 2026 Valparaíso del viernes 23 de enero al viernes 30 de enero"
	if got != want {
		t.Errorf("CreateMetadata =\n  %q\nwant\n  %q", got, want)
	}

	t.Run("nil metadata", func(t *testing.T) {
		got, err := CreateMetadata("X", "sec", "Dir", nil, "2026")
		if err != nil {
			t.Fatal(err)
		}
		if want := "SEC 2026 - X - DIR - Programación Excéntrico 2026"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("single date, no city", func(t *testing.T) {
		got, err := CreateMetadata("X", "sec", "Dir", &models.Metadata{Dates: [][]string{{"2026-01-23"}}}, "2026")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(got, "Excéntrico 2026 del viernes 23 de enero") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("invalid date", func(t *testing.T) {
		if _, err := CreateMetadata("X", "sec", "Dir", &models.Metadata{Dates: [][]string{{"23-01-2026"}}}, "2026"); err == nil {
			t.Error("expected error for invalid date")
		}
	})
}
