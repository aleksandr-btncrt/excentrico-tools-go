package utils

import "testing"

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Plain Title", "Plain Title"},
		{`a/b\c:d*e?f"g<h>i|j`, "a_b_c_d_e_f_g_h_i_j"},
		{"  ..Title..  ", "Title"},
		{"Many   inner\t spaces", "Many inner spaces"},
		{"Ñandú & Café", "Ñandú & Café"},
		{"", "unnamed_film"},
		{"   ", "unnamed_film"},
		{"...", "unnamed_film"},
	}
	for _, tt := range tests {
		if got := SanitizeFilename(tt.in); got != tt.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractFileIDFromURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://drive.google.com/drive/folders/1AbC-d_E", "1AbC-d_E"},
		{"https://drive.google.com/drive/u/0/folders/1AbC-d_E?usp=sharing", "1AbC-d_E"},
		{"https://drive.google.com/file/d/FILE_id-123/view", "FILE_id-123"},
		{"https://drive.google.com/open?id=OPEN_id", "OPEN_id"},
		{"https://docs.google.com/spreadsheets/d/SHEET_id/edit", "SHEET_id"},
		{"https://example.com/nothing-here", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := ExtractFileIDFromURL(tt.in); got != tt.want {
			t.Errorf("ExtractFileIDFromURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGetOptimizedImagePath(t *testing.T) {
	if got, want := GetOptimizedImagePath("films/x/Stills/a.png"), "films/x/Stills/a_web.jpg"; got != want {
		t.Errorf("GetOptimizedImagePath = %q, want %q", got, want)
	}
}

func TestIsImageFile(t *testing.T) {
	for _, m := range []string{"image/jpeg", "IMAGE/PNG", "image/webp"} {
		if !IsImageFile(m) {
			t.Errorf("IsImageFile(%q) = false, want true", m)
		}
	}
	for _, m := range []string{"", "video/mp4", "application/pdf"} {
		if IsImageFile(m) {
			t.Errorf("IsImageFile(%q) = true, want false", m)
		}
	}
}
