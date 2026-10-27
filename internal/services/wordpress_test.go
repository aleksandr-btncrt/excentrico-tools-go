package services

import (
	"io"
	"mime"
	"mime/multipart"
	"reflect"
	"strings"
	"testing"
)

func TestMatchCategory(t *testing.T) {
	cats := []*WordPressCategory{
		{ID: 1, Name: "2026"},
		{ID: 2, Name: "competencia-nacional-2026", Parent: 1},
		{ID: 3, Name: "Muestra Internacional 2026", Parent: 1},
		nil,
	}

	tests := []struct {
		name   string
		wantID int
	}{
		{"Competencia Nacional", 2},  // dashed form matches
		{"competencia nacional", 2},  // case-insensitive
		{"Muestra Internacional", 3}, // plain form fallback
		{"  Muestra Internacional ", 3},
		{"Retrospectiva", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := matchCategory(cats, tt.name)
		gotID := 0
		if got != nil {
			gotID = got.ID
		}
		if gotID != tt.wantID {
			t.Errorf("matchCategory(%q) = %d, want %d", tt.name, gotID, tt.wantID)
		}
	}
}

func TestBuildMultipart(t *testing.T) {
	parse := func(t *testing.T, title, alt string) map[string]string {
		t.Helper()
		body, contentType, err := buildMultipart(strings.NewReader("JPEGDATA"), "still_web.jpg", title, alt)
		if err != nil {
			t.Fatal(err)
		}
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "multipart/form-data" {
			t.Fatalf("content type %q: %v", contentType, err)
		}
		parts := map[string]string{}
		r := multipart.NewReader(body, params["boundary"])
		for {
			p, err := r.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(p)
			key := p.FormName()
			if p.FileName() != "" {
				key += ":" + p.FileName()
			}
			parts[key] = string(b)
		}
		return parts
	}

	got := parse(t, "Still", "A still")
	want := map[string]string{"file:still_web.jpg": "JPEGDATA", "title": "Still", "alt_text": "A still"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parts = %v, want %v", got, want)
	}

	got = parse(t, "", "")
	want = map[string]string{"file:still_web.jpg": "JPEGDATA"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("without title/alt: parts = %v, want %v", got, want)
	}
}

func TestCleanCategories(t *testing.T) {
	if got := cleanCategories(nil); got != nil {
		t.Errorf("cleanCategories(nil) = %v, want nil", got)
	}
	if got := cleanCategories([]int{0, 5, 0, 7}); !reflect.DeepEqual(got, []int{5, 7}) {
		t.Errorf("cleanCategories = %v, want [5 7]", got)
	}
	if got := cleanCategories([]int{0, 0}); len(got) != 0 {
		t.Errorf("cleanCategories(all zero) = %v, want empty", got)
	}
}
