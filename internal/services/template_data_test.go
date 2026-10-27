package services

import (
	"path/filepath"
	"strings"
	"testing"
)

// unsupportedTemplates lists files in templates/ that are kept exactly as
// exported from Divi but do not match the TemplateData schema. They are not
// edited by this tool; loading them must fail fast with a readable error
// instead of rendering a page with empty attributes.
var unsupportedTemplates = map[string]bool{
	"2025.json": true, // Divi module-keyed (et_pb_*) export
}

// TestTemplateFiles_Validate guarantees every shipped templates/<year>.json
// decodes strictly into TemplateData and is structurally complete, and that
// the known-unsupported ones are rejected rather than silently accepted.
func TestTemplateFiles_Validate(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "templates", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no template files found in templates/")
	}
	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			_, err := LoadTemplateData(f)
			if unsupportedTemplates[name] {
				if err == nil {
					t.Errorf("%s is listed as unsupported but now loads; remove it from unsupportedTemplates", name)
				}
				return
			}
			if err != nil {
				t.Errorf("%v", err)
			}
		})
	}
}

func TestTemplateData_ValidateListsAllMissing(t *testing.T) {
	err := TemplateData{}.Validate()
	if err == nil {
		t.Fatal("expected error for empty template")
	}
	for _, path := range []string{
		"menu.menu_id",
		"header.title_text_color",
		"header.subhead_text_color",
		"contenido.background_color_gradient_stops",
		"texto.header_4_text_color",
		"ndc.text.color",
		"footer.section.global_module",
	} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error %q does not mention %s", err, path)
		}
	}
}

func TestDecodeTemplateData_RejectsUnknownFields(t *testing.T) {
	// The legacy 2025 schema keyed by Divi module names must be rejected,
	// not silently decoded into an empty template.
	legacy := `{"et_pb_fullwidth_header": {"title_text_color": "#fff"}}`
	_, err := DecodeTemplateData(strings.NewReader(legacy))
	if err == nil || !strings.Contains(err.Error(), "et_pb_fullwidth_header") {
		t.Fatalf("expected unknown-field error naming the key, got %v", err)
	}
}
