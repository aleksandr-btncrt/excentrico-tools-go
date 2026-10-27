package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/sheet"
)

func sheetData() [][]any {
	return [][]any{
		{"TÍTULO ORIGINAL", nil, "EDICIÓN", "SECCIÓN", "DIRECCIÓN"},
		{"Meshes of the Afternoon", "x", "Excéntrico 2026", "Competencia", "Maya Deren"},
		{"La Jetée", "y", " excéntrico 2026 ", "Retrospectiva"},
		{"Old Film", "z", "Excéntrico 2025", "Competencia", "Someone"},
	}
}

func TestSheetOptionsFromConfig(t *testing.T) {
	if got := (&App{}).sheetOptions(); got != sheet.DefaultOptions() {
		t.Errorf("no config: %+v, want defaults", got)
	}
	a := &App{config: &config.Config{SheetTab: "2026", EditionPrefix: "Fest "}}
	got := a.sheetOptions()
	if got.DataRange() != "2026!A:ZZ" || got.Edition("2026") != "Fest 2026" {
		t.Errorf("configured options = %+v", got)
	}
}

func TestProcessFilms_DryRun(t *testing.T) {
	var events, progress bytes.Buffer
	prev := logger.SetDefault(logger.NewWithWriters("test", &events, &progress))
	defer logger.SetDefault(prev)

	t.Chdir(t.TempDir())
	a := &App{} // a dry run must not touch any service
	films := sheet.DefaultOptions().Select(sheet.Films(sheetData()), "2026", "")
	if err := a.processFilms(context.Background(), films, ProcessOptions{Year: "2026", DryRun: true}); err != nil {
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

func TestProcessFilms_StopsWhenCancelled(t *testing.T) {
	var progress bytes.Buffer
	prev := logger.SetDefault(logger.NewWithWriters("test", &bytes.Buffer{}, &progress))
	defer logger.SetDefault(prev)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &App{}
	err := a.processFilms(ctx, sheet.Films(sheetData()), ProcessOptions{DryRun: true})
	if err == nil || !strings.Contains(progress.String(), "Interrupted after 0 of 3 films") {
		t.Errorf("err = %v, progress = %q; want interruption before the first film", err, progress.String())
	}
}
