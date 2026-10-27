package film

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/services"
	"excentrico-tools-go/internal/sheet"
)

// testFilm builds a sheet row with the columns the pipeline reads.
func testFilm(title, director, folderID string) *sheet.Film {
	headers := []string{sheet.ColTituloOriginal, sheet.ColDireccion, sheet.ColSeccion, sheet.ColEnlaces}
	return sheet.RowToFilm(headers, []any{title, director, "Competencia", "https://drive.google.com/drive/folders/" + folderID})
}

type pipeline struct {
	drive   *fakeDrive
	resizer *fakeResizer
	wp      *fakeWordPress
	store   *fakeStore
	proc    *Processor
	baseDir string
}

func newPipeline(t *testing.T) *pipeline {
	t.Helper()
	prev := logger.SetDefault(logger.NewWithWriters("test", &bytes.Buffer{}, &bytes.Buffer{}))
	t.Cleanup(func() { logger.SetDefault(prev) })

	p := &pipeline{
		drive:   &fakeDrive{},
		resizer: &fakeResizer{},
		wp:      newFakeWordPress(),
		store:   newFakeStore(),
		baseDir: t.TempDir(),
	}
	p.proc = NewProcessor(p.drive, p.resizer, p.wp, services.NewDiviTemplateService(), p.store)
	return p
}

func (p *pipeline) run(t *testing.T, f *sheet.Film) Result {
	t.Helper()
	res, err := p.proc.ProcessSingleFilm(context.Background(), f, p.baseDir, "2026", "caption", services.TemplateData{})
	if err != nil {
		t.Fatalf("ProcessSingleFilm(%q): %v", f.Title(), err)
	}
	return res
}

func countTitle(titles []string, want string) int {
	n := 0
	for _, t := range titles {
		if t == want {
			n++
		}
	}
	return n
}

// A director shared by two films is uploaded once; the second film reuses
// the cached media (regression: duplicate director images per film).
func TestProcessSingleFilm_DirectorDeduplicatedAcrossFilms(t *testing.T) {
	p := newPipeline(t)
	newFilmFolder(p.drive, "rootA", "Maya Deren")
	newFilmFolder(p.drive, "rootB", "Maya Deren")

	resA := p.run(t, testFilm("Film A", "Maya Deren", "rootA"))
	resB := p.run(t, testFilm("Film B", "Maya Deren", "rootB"))

	if got := countTitle(p.wp.uploads, "Maya Deren"); got != 1 {
		t.Errorf("director uploaded %d times, want 1 (uploads: %q)", got, p.wp.uploads)
	}
	// Each film still uploads its own still and references the director image.
	if resA.Images != 2 || resB.Images != 2 {
		t.Errorf("images = %d, %d; want 2 each", resA.Images, resB.Images)
	}
	if p.wp.creates != 2 {
		t.Errorf("CreatePost called %d times, want 2", p.wp.creates)
	}
	// One batched media lookup per film (plan 6.2).
	if p.wp.fetches != 2 {
		t.Errorf("FetchMedia called %d times, want 2 (one per film)", p.wp.fetches)
	}
	if len(p.drive.downloads) != 4 || p.resizer.calls != 4 {
		t.Errorf("downloads = %d, resizes = %d; want 4 each", len(p.drive.downloads), p.resizer.calls)
	}

	var cache map[string]services.DirectorMediaCacheEntry
	if err := services.GetDirectorMediaCache(p.store, &cache); err != nil {
		t.Fatalf("director cache: %v", err)
	}
	if _, ok := cache["maya deren"]; !ok {
		t.Errorf("director cache = %v, want an entry for \"maya deren\"", cache)
	}
}

// Re-processing a film updates its existing post instead of creating a new
// one, and skips files that are already downloaded and uploaded.
func TestProcessSingleFilm_UpdatesExistingPost(t *testing.T) {
	p := newPipeline(t)
	newFilmFolder(p.drive, "rootA", "Maya Deren")
	f := testFilm("Film A", "Maya Deren", "rootA")

	first := p.run(t, f)
	uploads := len(p.wp.uploads)
	second := p.run(t, f)

	if p.wp.creates != 1 {
		t.Errorf("CreatePost called %d times, want 1", p.wp.creates)
	}
	if len(p.wp.updates) != 1 || p.wp.updates[0] != first.PostID {
		t.Errorf("UpdatePost calls = %v, want [%d]", p.wp.updates, first.PostID)
	}
	if second.PostID != first.PostID {
		t.Errorf("post ID changed: %d -> %d", first.PostID, second.PostID)
	}
	if len(p.wp.uploads) != uploads {
		t.Errorf("second run uploaded %d more media, want 0", len(p.wp.uploads)-uploads)
	}
	if len(p.drive.downloads) != 2 {
		t.Errorf("downloads = %d, want 2 (files on disk are not re-downloaded)", len(p.drive.downloads))
	}

	var meta struct {
		PostID int `json:"post_id"`
	}
	if err := services.GetWordPressMetadata(p.store, "Film A", &meta); err != nil {
		t.Fatalf("wordpress metadata: %v", err)
	}
	if meta.PostID != first.PostID {
		t.Errorf("stored post_id = %d, want %d", meta.PostID, first.PostID)
	}

	filmDir := filepath.Join(p.baseDir, "Film A")
	for _, name := range []string{"metadata.json", "divi_template.json"} {
		if _, err := os.Stat(filepath.Join(filmDir, name)); err != nil {
			t.Errorf("%s not written: %v", name, err)
		}
	}
}

// divi_template.json embeds every cached image only when EmbedImages is set;
// otherwise "encoded" is empty and nothing is downloaded.
func TestProcessSingleFilm_EmbedImages(t *testing.T) {
	for _, embed := range []bool{false, true} {
		p := newPipeline(t)
		divi := services.NewDiviTemplateService()
		divi.EmbedImages = embed
		p.proc = NewProcessor(p.drive, p.resizer, p.wp, divi, p.store)
		newFilmFolder(p.drive, "rootA", "Maya Deren")

		res := p.run(t, testFilm("Film A", "Maya Deren", "rootA"))

		b, err := os.ReadFile(filepath.Join(p.baseDir, "Film A", "divi_template.json"))
		if err != nil {
			t.Fatal(err)
		}
		var file struct {
			Images map[string]services.DiviImageData `json:"images"`
		}
		if err := json.Unmarshal(b, &file); err != nil {
			t.Fatal(err)
		}
		if len(file.Images) != res.Images {
			t.Errorf("embed=%v: %d images in file, want %d", embed, len(file.Images), res.Images)
		}
		for u, img := range file.Images {
			if (img.Encoded != "") != embed {
				t.Errorf("embed=%v: image %s encoded=%q", embed, u, img.Encoded)
			}
		}
		wantDownloads := 0
		if embed {
			wantDownloads = res.Images
		}
		if p.wp.downloads != wantDownloads {
			t.Errorf("embed=%v: %d downloads, want %d", embed, p.wp.downloads, wantDownloads)
		}
	}
}

// Without an ENLACES URL the Drive step is skipped and nothing is uploaded.
func TestProcessSingleFilm_NoDriveFolder(t *testing.T) {
	p := newPipeline(t)
	f := sheet.RowToFilm([]string{sheet.ColTituloOriginal}, []any{"Solo"})

	res := p.run(t, f)
	if res.Images != 0 || len(p.drive.downloads) != 0 || len(p.wp.uploads) != 0 {
		t.Errorf("res = %+v, downloads = %d, uploads = %d; want nothing", res, len(p.drive.downloads), len(p.wp.uploads))
	}
	if p.wp.creates != 1 {
		t.Errorf("CreatePost called %d times, want 1", p.wp.creates)
	}
}
