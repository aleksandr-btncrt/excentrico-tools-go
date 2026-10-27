package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"excentrico-tools-go/internal/config"
)

// mediaServer serves /wp-json/wp/v2/media?include=...&page=N with at most
// pageSize items per page (and X-WP-TotalPages), plus /wp/v2/media/<id>.
// Every request path+query is recorded.
type mediaServer struct {
	mu        sync.Mutex
	requests  []string
	pageSize  int
	failBatch bool
}

func (m *mediaServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.requests = append(m.requests, r.URL.RequestURI())
	m.mu.Unlock()

	media := func(id int) map[string]any {
		return map[string]any{"id": id, "source_url": fmt.Sprintf("https://example.test/uploads/img%d.jpg", id)}
	}
	w.Header().Set("Content-Type", "application/json")

	if rest, ok := strings.CutPrefix(r.URL.Path, "/wp-json/wp/v2/media/"); ok {
		id, _ := strconv.Atoi(rest)
		_ = json.NewEncoder(w).Encode(media(id))
		return
	}
	if m.failBatch {
		http.Error(w, "batch disabled", http.StatusBadRequest)
		return
	}

	var ids []int
	for _, s := range strings.Split(r.URL.Query().Get("include"), ",") {
		if id, err := strconv.Atoi(s); err == nil {
			ids = append(ids, id)
		}
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	totalPages := (len(ids) + m.pageSize - 1) / m.pageSize
	w.Header().Set("X-WP-TotalPages", strconv.Itoa(totalPages))
	start := (page - 1) * m.pageSize
	end := min(start+m.pageSize, len(ids))
	items := []map[string]any{}
	for _, id := range ids[start:end] {
		items = append(items, media(id))
	}
	_ = json.NewEncoder(w).Encode(items)
}

func (m *mediaServer) start(t *testing.T) *WordPressService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(m.handler))
	t.Cleanup(srv.Close)
	return NewWordPressService(config.WordPressConfig{BaseURL: srv.URL, Username: "u", Password: "p"})
}

func idsRange(from, to int) []int {
	var ids []int
	for i := from; i <= to; i++ {
		ids = append(ids, i)
	}
	return ids
}

// 150 unique IDs (plus duplicates and a zero) are fetched in two ?include=
// batches of at most 100; a batch spread over two pages is followed.
func TestFetchMedia_ChunksAndPaginates(t *testing.T) {
	m := &mediaServer{pageSize: 60}
	s := m.start(t)

	ids := append(idsRange(1, 150), 1, 2, 0)
	cache, err := s.FetchMedia(context.Background(), ids)
	if err != nil {
		t.Fatalf("FetchMedia: %v", err)
	}
	if cache.Len() != 150 {
		t.Errorf("cache.Len() = %d, want 150", cache.Len())
	}
	if got := cache.IDs(); got[0] != 1 || got[149] != 150 {
		t.Errorf("IDs not in request order: first %d, last %d", got[0], got[149])
	}
	if m2, ok := cache.Get(42); !ok || m2.SourceURL != "https://example.test/uploads/img42.jpg" {
		t.Errorf("Get(42) = %+v, %v", m2, ok)
	}

	// Batch 1: 100 IDs = pages 1-2; batch 2: 50 IDs = page 1.
	if len(m.requests) != 3 {
		t.Fatalf("requests = %d, want 3: %q", len(m.requests), m.requests)
	}
	for _, r := range m.requests {
		if !strings.HasPrefix(r, "/wp-json/wp/v2/media?") || !strings.Contains(r, "per_page=100") {
			t.Errorf("unexpected request %q", r)
		}
	}
	if !strings.Contains(m.requests[1], "page=2") {
		t.Errorf("second request %q, want page=2 of the first batch", m.requests[1])
	}
}

// When ?include= fails, the chunk degrades to one GET /media/<id> per ID.
func TestFetchMedia_FallsBackToPerID(t *testing.T) {
	m := &mediaServer{pageSize: 100, failBatch: true}
	s := m.start(t)

	cache, err := s.FetchMedia(context.Background(), []int{7, 8, 9})
	if err != nil {
		t.Fatalf("FetchMedia: %v", err)
	}
	if cache.Len() != 3 {
		t.Errorf("cache.Len() = %d, want 3", cache.Len())
	}
	if len(m.requests) != 4 { // 1 failed batch + 3 single lookups
		t.Errorf("requests = %d, want 4: %q", len(m.requests), m.requests)
	}
}

func TestFetchMedia_NoIDsNoRequests(t *testing.T) {
	m := &mediaServer{pageSize: 100}
	s := m.start(t)
	cache, err := s.FetchMedia(context.Background(), nil)
	if err != nil || cache.Len() != 0 || len(m.requests) != 0 {
		t.Errorf("cache.Len()=%d err=%v requests=%d; want 0, nil, 0", cache.Len(), err, len(m.requests))
	}
}

func TestFetchMedia_CancelledContext(t *testing.T) {
	m := &mediaServer{pageSize: 100}
	s := m.start(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.FetchMedia(ctx, []int{1}); err == nil {
		t.Error("FetchMedia with cancelled context: want error")
	}
}

func TestMediaCache_NilIsEmpty(t *testing.T) {
	var c *MediaCache
	if _, ok := c.Get(1); ok || c.Len() != 0 || c.IDs() != nil {
		t.Error("nil *MediaCache should behave as empty")
	}
}

func TestMediaFilenameStem(t *testing.T) {
	tests := map[string]string{
		"": "",
		"https://x.test/uploads/Poster_Final.jpg": "poster_final",
		"https://x.test/uploads/a.b.png":          "a.b",
		"noslash.JPEG":                            "noslash",
		"https://x.test/uploads/noext":            "noext",
	}
	for in, want := range tests {
		if got := MediaFilenameStem(in); got != want {
			t.Errorf("MediaFilenameStem(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMediaMatchesAny(t *testing.T) {
	m := &WordPressMedia{
		Title:     WordPressRenderedField{Rendered: "Still"},
		AltText:   "",
		SourceURL: "https://x.test/uploads/Film_POSTER.jpg",
	}
	if !MediaMatchesAny(m, []string{"poster"}) {
		t.Error("want match on file name")
	}
	if MediaMatchesAny(m, []string{"fondo"}) {
		t.Error("unexpected match")
	}
}
