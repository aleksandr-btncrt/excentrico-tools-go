package main

import (
	"testing"

	"excentrico-tools-go/internal/services"
)

func testMenus() []*services.WordPressMenu {
	return []*services.WordPressMenu{
		{ID: 137, Name: "Menu 2025", Slug: "menu-2025"},
		{ID: 163, Name: "Menu 2026", Slug: "menu-2026"},
		{ID: 5, Name: "Principal", Slug: "principal"},
	}
}

func TestFindMenu(t *testing.T) {
	menus := testMenus()
	tests := []struct {
		choice string
		wantID int
	}{
		{"2", 163},
		{"menu-2025", 137},
		{"PRINCIPAL", 5},
		{" Menu 2026 ", 163},
		{"0", 0},
		{"4", 0},
		{"nope", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := findMenu(menus, tt.choice)
		gotID := 0
		if got != nil {
			gotID = got.ID
		}
		if gotID != tt.wantID {
			t.Errorf("findMenu(%q) = %d, want %d", tt.choice, gotID, tt.wantID)
		}
	}
}

func TestFilterMenusByYear(t *testing.T) {
	menus := testMenus()

	got, matched := filterMenusByYear(menus, "2026")
	if !matched || len(got) != 1 || got[0].ID != 163 {
		t.Errorf("filter 2026 = %v (matched=%v)", got, matched)
	}

	got, matched = filterMenusByYear(menus, "1999")
	if matched || len(got) != len(menus) {
		t.Errorf("no match should return all menus, got %d (matched=%v)", len(got), matched)
	}

	got, matched = filterMenusByYear(menus, "")
	if matched || len(got) != len(menus) {
		t.Errorf("empty year should return all menus, got %d (matched=%v)", len(got), matched)
	}
}
