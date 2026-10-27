package wordpress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/services"
	"excentrico-tools-go/internal/sheet"
)

// memStore is an in-memory services.MetadataStore (JSON round-trip).
type memStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (s *memStore) SaveMetadata(filmID, metadataType string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[filmID+"/"+metadataType] = b
	return nil
}

func (s *memStore) GetMetadata(filmID, metadataType string, dest any) error {
	s.mu.Lock()
	b, ok := s.data[filmID+"/"+metadataType]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %s/%s", services.ErrMetadataNotFound, filmID, metadataType)
	}
	return json.Unmarshal(b, dest)
}

// TestCreateOrUpdate_MediaRequestBudget drives the real WordPressService
// against an httptest server for a film with 3 directors and 20 images and
// checks that the post step issues at most 5 GET /wp/v2/media* requests
// (plan 6.7; previously 200+ for this shape).
func TestCreateOrUpdate_MediaRequestBudget(t *testing.T) {
	prev := logger.SetDefault(logger.NewWithWriters("test", &bytes.Buffer{}, &bytes.Buffer{}))
	t.Cleanup(func() { logger.SetDefault(prev) })

	directors := []string{"Maya Deren", "Agnès Varda", "Chantal Akerman"}
	var (
		mu            sync.Mutex
		mediaRequests []string
		downloads     int
	)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/media"):
			mu.Lock()
			mediaRequests = append(mediaRequests, r.URL.RequestURI())
			mu.Unlock()
			var items []map[string]any
			for _, s := range strings.Split(r.URL.Query().Get("include"), ",") {
				id, err := strconv.Atoi(s)
				if err != nil {
					continue
				}
				title := fmt.Sprintf("still %d", id)
				if id <= len(directors) {
					title = directors[id-1]
				}
				items = append(items, map[string]any{
					"id":         id,
					"title":      map[string]string{"rendered": title},
					"source_url": fmt.Sprintf("%s/uploads/img%d.jpg", srv.URL, id),
				})
			}
			w.Header().Set("X-WP-TotalPages", "1")
			_ = json.NewEncoder(w).Encode(items)
		case strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/categories"):
			_, _ = w.Write([]byte("[]"))
		case r.Method == http.MethodPost && r.URL.Path == "/wp-json/wp/v2/project":
			_, _ = w.Write([]byte(`{"id":500,"slug":"film","status":"draft","title":{"rendered":"Film"}}`))
		case strings.HasPrefix(r.URL.Path, "/uploads/"):
			mu.Lock()
			downloads++
			mu.Unlock()
			_, _ = w.Write([]byte("jpeg"))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	wp := services.NewWordPressService(config.WordPressConfig{BaseURL: srv.URL, Username: "u", Password: "p"})
	divi := services.NewDiviTemplateService() // embeds images: worst case
	store := &memStore{data: map[string][]byte{}}

	headers := []string{sheet.ColTituloOriginal, sheet.ColDireccion, sheet.ColSeccion, sheet.ColMultiDir}
	film := sheet.RowToFilm(headers, []any{"Film", strings.Join(directors, ", "), "Competencia", "SI"})
	if got := len(film.Directors()); got != 3 {
		t.Fatalf("test film has %d directors, want 3", got)
	}

	var imageIDs []int
	for id := 1; id <= 20; id++ {
		imageIDs = append(imageIDs, id)
	}
	filmDir := t.TempDir()

	postID, err := CreateOrUpdateWordPressProject(context.Background(), wp, divi, store, filmDir, film, "2026", imageIDs, services.TemplateData{})
	if err != nil {
		t.Fatalf("CreateOrUpdateWordPressProject: %v", err)
	}
	if postID != 500 {
		t.Errorf("postID = %d, want 500", postID)
	}

	if n := len(mediaRequests); n == 0 || n > 5 {
		t.Errorf("GET /wp/v2/media* requests = %d, want 1..5: %q", n, mediaRequests)
	}
	if downloads != 20 {
		t.Errorf("image downloads = %d, want 20 (one per image with embedding on)", downloads)
	}

	b, err := os.ReadFile(filepath.Join(filmDir, "divi_template.json"))
	if err != nil {
		t.Fatalf("divi_template.json: %v", err)
	}
	for _, d := range directors {
		if !bytes.Contains(b, []byte(strings.ToUpper(d))) && !bytes.Contains(b, []byte(d)) {
			t.Errorf("divi_template.json does not mention director %q", d)
		}
	}
	for i := 1; i <= len(directors); i++ {
		if !bytes.Contains(b, []byte(fmt.Sprintf("/uploads/img%d.jpg", i))) {
			t.Errorf("director image img%d.jpg not referenced in divi_template.json", i)
		}
	}
}
