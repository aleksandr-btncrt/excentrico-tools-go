package services

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// updateGolden regenerates the golden files instead of comparing against them:
//
//	go test ./internal/services -run TestGolden -update
//
// Only use it for intentional, reviewed output changes.
var updateGolden = flag.Bool("update", false, "rewrite golden files in testdata/")

func loadTemplateFixture(t *testing.T, year string) TemplateData {
	t.Helper()
	cfg, err := LoadTemplateData(filepath.Join("..", "..", "templates", year+".json"))
	if err != nil {
		t.Fatalf("load template: %v", err)
	}
	return cfg
}

// goldenFilm is a hand-written fixture covering escaping, accents, multiple
// directors (one without an image) and every individual credit field.
func goldenFilm() *DiviFilmTemplate {
	return &DiviFilmTemplate{
		Title:           `Las "Chicas" & el Mar`,
		Country:         "Chile",
		Year:            "2025",
		Duration:        parseDuration("1:12:30"),
		BackgroundImage: "https://example.com/wp-content/uploads/bg_web.jpg",
		Directors: []DirectorInfo{
			{Name: "María José Ríos", Bio: "Directora <chilena> de cine & video.", ImageURL: "https://example.com/wp-content/uploads/maria-jose-rios_web.jpg"},
			{Name: "Alex Ray", Bio: "Artista visual.", ImageURL: ""},
		},
		Synopsis:     "Una película sobre el mar, las olas & la memoria.",
		ContentNotes: "Desnudez, violencia",
		Credits: Credits{
			Production:  "Productora Uno",
			Script:      "Guionista Dos",
			Photography: "Fotógrafa Tres",
			ArtDesign:   "Arte Cuatro",
			SoundMusic:  "Sonido Cinco",
			Editing:     "Editor Seis",
			Cast:        "Actriz Siete (ella)",
		},
		ImageGalleryIds: []int{101, 102, 103},
		GalleryMediaIds: "101,102,103",
	}
}

// goldenFilmOtherCredits exercises the "Otros créditos" branch and the
// empty content-notes branch.
func goldenFilmOtherCredits() *DiviFilmTemplate {
	f := goldenFilm()
	f.Directors = f.Directors[:1]
	f.ContentNotes = ""
	f.Credits = Credits{OtherCredits: "Producción: Uno. Guión: Dos. Montaje: <Tres>."}
	return f
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create it)", path, err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s (len got=%d want=%d); run `go test ./internal/services -run TestGolden -update` only for intentional changes", path, len(got), len(want))
	}
}

func TestGolden2026(t *testing.T) {
	cfg := loadTemplateFixture(t, "2026")
	s := NewDiviTemplateService()
	assertGolden(t, "golden_2026.txt", s.GenerateDiviShortcodeTemplate(goldenFilm(), "2026", cfg))
}

func TestGolden2026OtherCredits(t *testing.T) {
	cfg := loadTemplateFixture(t, "2026")
	s := NewDiviTemplateService()
	assertGolden(t, "golden_2026_other_credits.txt", s.GenerateDiviShortcodeTemplate(goldenFilmOtherCredits(), "2026", cfg))
}
