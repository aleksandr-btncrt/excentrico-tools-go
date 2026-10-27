package utils

import "testing"

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
		if got := CellString(tt.in); got != tt.want {
			t.Errorf("CellString(%#v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
