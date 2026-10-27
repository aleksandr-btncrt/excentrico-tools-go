package film

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCaption(t *testing.T) {
	dir := t.TempDir()
	caption := `COMPETENCIA 2026 - "Film" - MAYA DEREN & YOKO ONO - Programación Excéntrico 2026`
	if err := WriteCaption(dir, caption); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got FilmMetadata
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("metadata.json is not valid JSON: %v\n%s", err, data)
	}
	if got.Caption != caption {
		t.Errorf("caption = %q, want %q", got.Caption, caption)
	}
}

func TestWriteCaptionMissingDir(t *testing.T) {
	if err := WriteCaption(filepath.Join(t.TempDir(), "missing"), "x"); err == nil {
		t.Error("expected error when the film directory does not exist")
	}
}
