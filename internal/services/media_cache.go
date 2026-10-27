package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// mediaBatchSize is the WordPress REST maximum for per_page, and the number
// of IDs sent per ?include= request.
const mediaBatchSize = 100

// maxMediaBatchPages bounds pagination of one batch (a batch of 100 IDs at
// per_page=100 is normally a single page).
const maxMediaBatchPages = 10

// MediaCache holds media items fetched once per film so the template,
// featured-image and director selectors do not each call GET /media/<id>.
// A nil *MediaCache behaves as an empty cache.
type MediaCache struct {
	byID  map[int]*WordPressMedia
	order []int
}

// NewMediaCache builds a cache from items, keeping their order.
func NewMediaCache(items []*WordPressMedia) *MediaCache {
	c := &MediaCache{byID: make(map[int]*WordPressMedia, len(items))}
	for _, m := range items {
		c.add(m)
	}
	return c
}

func (c *MediaCache) add(m *WordPressMedia) {
	if m == nil || m.ID == 0 {
		return
	}
	if _, ok := c.byID[m.ID]; !ok {
		c.order = append(c.order, m.ID)
	}
	c.byID[m.ID] = m
}

// Get returns the media item with id.
func (c *MediaCache) Get(id int) (*WordPressMedia, bool) {
	if c == nil {
		return nil, false
	}
	m, ok := c.byID[id]
	return m, ok
}

// Len returns the number of cached items.
func (c *MediaCache) Len() int {
	if c == nil {
		return 0
	}
	return len(c.byID)
}

// IDs returns the cached IDs in fetch order.
func (c *MediaCache) IDs() []int {
	if c == nil {
		return nil
	}
	return append([]int(nil), c.order...)
}

// MediaFilenameStem returns the lower-case file name of a media URL without
// its extension, e.g. ".../Poster_Final.jpg" -> "poster_final".
func MediaFilenameStem(sourceURL string) string {
	if sourceURL == "" {
		return ""
	}
	name := sourceURL
	if i := strings.LastIndex(name, "/"); i != -1 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(name, path.Ext(name))
	return strings.ToLower(name)
}

// FetchMedia loads the given media IDs with
// GET /wp/v2/media?include=<ids>&per_page=100, chunked by 100 IDs and
// following X-WP-TotalPages. If a batch request fails, that chunk falls back
// to one GetMedia per ID. IDs that cannot be loaded are left out of the
// cache (callers treat them like a failed GetMedia). Duplicates and zero IDs
// are ignored. The error is non-nil only when the context is cancelled.
func (s *WordPressService) FetchMedia(ctx context.Context, ids []int) (*MediaCache, error) {
	cache := NewMediaCache(nil)
	unique := make([]int, 0, len(ids))
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id != 0 && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	for start := 0; start < len(unique); start += mediaBatchSize {
		end := min(start+mediaBatchSize, len(unique))
		chunk := unique[start:end]

		items, err := s.fetchMediaBatch(ctx, chunk)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return cache, ctxErr
			}
			// Batch endpoint unavailable (plugin, proxy, permissions):
			// degrade to per-ID requests for this chunk.
			for _, id := range chunk {
				if m, err := s.GetMedia(ctx, id); err == nil {
					cache.add(m)
				} else if ctxErr := ctx.Err(); ctxErr != nil {
					return cache, ctxErr
				}
			}
			continue
		}
		for _, m := range items {
			if seen[m.ID] {
				cache.add(m)
			}
		}
	}
	return cache, nil
}

// fetchMediaBatch requests up to mediaBatchSize IDs, following pagination.
func (s *WordPressService) fetchMediaBatch(ctx context.Context, ids []int) ([]*WordPressMedia, error) {
	include := make([]string, len(ids))
	for i, id := range ids {
		include[i] = strconv.Itoa(id)
	}

	var all []*WordPressMedia
	for page, totalPages := 1, 1; page <= totalPages; page++ {
		q := url.Values{}
		q.Set("include", strings.Join(include, ","))
		q.Set("per_page", strconv.Itoa(mediaBatchSize))
		q.Set("orderby", "include")
		q.Set("page", strconv.Itoa(page))

		resp, err := s.makeRequest(ctx, "GET", "/wp/v2/media?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		var items []*WordPressMedia
		err = json.NewDecoder(resp.Body).Decode(&items)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode media batch: %w", err)
		}
		all = append(all, items...)

		if tp, err := strconv.Atoi(resp.Header.Get("X-WP-TotalPages")); err == nil && tp > totalPages && tp <= maxMediaBatchPages {
			totalPages = tp
		}
	}
	return all, nil
}
