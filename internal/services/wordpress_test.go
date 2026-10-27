package services

import (
	"reflect"
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
