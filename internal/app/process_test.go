package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"excentrico-tools-go/internal/logger"
)

func sheetData() [][]any {
	return [][]any{
		{"TÍTULO ORIGINAL", nil, "EDICIÓN", "SECCIÓN", "DIRECCIÓN"},
		{"Meshes of the Afternoon", "x", "Excéntrico 2026", "Competencia", "Maya Deren"},
		{"La Jetée", "y", " excéntrico 2026 ", "Retrospectiva"},
		{"Old Film", "z", "Excéntrico 2025", "Competencia", "Someone"},
	}
}

func TestRowsToObjects_KeepsColumnsAligned(t *testing.T) {
	objs := rowsToObjects(sheetData())
	if len(objs) != 3 {
		t.Fatalf("got %d objects, want 3", len(objs))
	}
	// The empty second header must not shift later columns.
	if got := objs[0]["EDICIÓN"]; got != "Excéntrico 2026" {
		t.Errorf("EDICIÓN = %v, want Excéntrico 2026", got)
	}
	if _, ok := objs[0][""]; ok {
		t.Error("empty header should not become a key")
	}
	// Missing trailing cells become "".
	if got := objs[1]["DIRECCIÓN"]; got != "" {
		t.Errorf("missing cell = %v, want empty string", got)
	}
}

func TestFilterRows(t *testing.T) {
	objs := rowsToObjects(sheetData())
	titles := func(rows []map[string]any) string {
		var ts []string
		for _, r := range rows {
			ts = append(ts, filmTitle(r))
		}
		return strings.Join(ts, "|")
	}
	tests := []struct {
		year, film, want string
	}{
		{"", "", "Meshes of the Afternoon|La Jetée|Old Film"},
		{"2026", "", "Meshes of the Afternoon|La Jetée"},
		{"2026", "la jetée", "La Jetée"},
		{"2026", "Old Film", ""},
		{"", "  old film ", "Old Film"},
	}
	for _, tt := range tests {
		if got := titles(filterRows(objs, tt.year, tt.film)); got != tt.want {
			t.Errorf("filterRows(year=%q, film=%q) = %q, want %q", tt.year, tt.film, got, tt.want)
		}
	}
}

func TestProcessFilteredObjects_DryRun(t *testing.T) {
	var events, progress bytes.Buffer
	prev := logger.SetDefault(logger.NewWithWriters("test", &events, &progress))
	defer logger.SetDefault(prev)

	t.Chdir(t.TempDir())
	a := &App{} // a dry run must not touch any service
	films := filterRows(rowsToObjects(sheetData()), "2026", "")
	if err := a.processFilteredObjects(context.Background(), films, ProcessOptions{Year: "2026", DryRun: true}); err != nil {
		t.Fatalf("dry run: %v", err)
	}

	out := progress.String()
	for _, want := range []string{
		`[1/2] "Meshes of the Afternoon" → would process`,
		`[2/2] "La Jetée" → would process`,
		"Done: 2 films, 2 succeeded, 0 failed.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("progress lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "{") {
		t.Errorf("JSON events leaked into progress output:\n%s", out)
	}
}

func TestProcessFilteredObjects_StopsWhenCancelled(t *testing.T) {
	var progress bytes.Buffer
	prev := logger.SetDefault(logger.NewWithWriters("test", &bytes.Buffer{}, &progress))
	defer logger.SetDefault(prev)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &App{}
	err := a.processFilteredObjects(ctx, rowsToObjects(sheetData()), ProcessOptions{DryRun: true})
	if err == nil || !strings.Contains(progress.String(), "Interrupted after 0 of 3 films") {
		t.Errorf("err = %v, progress = %q; want interruption before the first film", err, progress.String())
	}
}
