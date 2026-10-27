package services

import (
	"strings"
	"testing"
)

// Edge cases of the component templates that the golden files do not cover.

func TestCreditsComponent_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		c    CreditsComponent
		want string
	}{
		{"empty", CreditsComponent{}, ""},
		{
			"other credits with no sentences renders nothing",
			CreditsComponent{Credits: Credits{OtherCredits: " . . ", Script: "ignored"}},
			"",
		},
		{
			"single field, no directors",
			CreditsComponent{Credits: Credits{Script: "A & B"}},
			"<p><strong>Guión:</strong> A &amp; B</p>",
		},
		{
			"directors joined and escaped",
			CreditsComponent{Directors: []DirectorInfo{{Name: "Ana"}, {Name: "<Bo>"}}},
			"<p><strong>Dirección:</strong> Ana, &lt;Bo&gt;</p>",
		},
		{
			"other credits split on dots",
			CreditsComponent{Credits: Credits{OtherCredits: "Uno. Dos"}},
			"<p>Uno<br>Dos</p>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.Render(); got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContentNotesComponent_EmptyRendersNothing(t *testing.T) {
	if got := (&ContentNotesComponent{}).Render(); got != "" {
		t.Errorf("Render() = %q, want empty", got)
	}
}

func TestDirectorComponent_NoDirectorsAndMissingImage(t *testing.T) {
	if got := (&DirectorComponent{}).Render(); got != "" {
		t.Errorf("no directors: Render() = %q, want empty", got)
	}
	got := (&DirectorComponent{Directors: []DirectorInfo{{Name: "Ana"}}}).Render()
	if !strings.Contains(got, `[et_pb_image src="" alt="Ana" title_text="Ana"`) {
		t.Errorf("director without image: %q", got)
	}
}
