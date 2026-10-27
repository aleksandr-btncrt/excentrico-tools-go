package film

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	gdrive "google.golang.org/api/drive/v3"

	"excentrico-tools-go/internal/services"
)

// In-memory fakes for the processor's dependencies. They let the whole
// per-film pipeline run in a temp directory with no network access.

// fakeStore is an in-memory services.MetadataStore. Values are stored as
// JSON, like Turso, so the decode path is exercised too.
type fakeStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newFakeStore() *fakeStore { return &fakeStore{data: map[string][]byte{}} }

func (s *fakeStore) SaveMetadata(filmID, metadataType string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[filmID+"/"+metadataType] = b
	return nil
}

func (s *fakeStore) GetMetadata(filmID, metadataType string, dest any) error {
	s.mu.Lock()
	b, ok := s.data[filmID+"/"+metadataType]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w for film '%s' type '%s'", services.ErrMetadataNotFound, filmID, metadataType)
	}
	return json.Unmarshal(b, dest)
}

// fakeDrive serves a folder tree: folders maps a folder ID to its children.
// Downloads write the file ID as the file content.
type fakeDrive struct {
	folders   map[string][]*gdrive.File
	downloads []string
}

const folderMime = "application/vnd.google-apps.folder"

// newFilmFolder returns a Drive tree for one film:
// <root>/Dir/<director>.jpg and <root>/Stills/still1.jpg.
func newFilmFolder(d *fakeDrive, root, director string) {
	if d.folders == nil {
		d.folders = map[string][]*gdrive.File{}
	}
	d.folders[root] = []*gdrive.File{
		{Id: root + "-dir", Name: "Dir", MimeType: folderMime},
		{Id: root + "-stills", Name: "Stills", MimeType: folderMime},
	}
	d.folders[root+"-dir"] = []*gdrive.File{{Id: root + "-d1", Name: director + ".jpg", MimeType: "image/jpeg"}}
	d.folders[root+"-stills"] = []*gdrive.File{{Id: root + "-s1", Name: "still1.jpg", MimeType: "image/jpeg"}}
}

func (d *fakeDrive) ListFiles(_ context.Context, folderID string) ([]*gdrive.File, error) {
	files, ok := d.folders[folderID]
	if !ok {
		return nil, fmt.Errorf("folder %s not found", folderID)
	}
	return files, nil
}

func (d *fakeDrive) DownloadFile(_ context.Context, fileID, destinationPath string) error {
	d.downloads = append(d.downloads, fileID)
	return os.WriteFile(destinationPath, []byte(fileID), 0o644)
}

// fakeResizer copies the input to the output path.
type fakeResizer struct{ calls int }

func (r *fakeResizer) ResizeImage(inputPath, outputPath string) error {
	r.calls++
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, b, 0o644)
}

// fakeWordPress keeps media and posts in memory and records every call.
type fakeWordPress struct {
	nextID  int
	media   map[int]*services.WordPressMedia
	posts   map[int]*services.WordPressPost
	uploads []string // titles of uploaded media
	creates int
	updates []int // post IDs passed to UpdatePost
}

func newFakeWordPress() *fakeWordPress {
	return &fakeWordPress{
		nextID: 100,
		media:  map[int]*services.WordPressMedia{},
		posts:  map[int]*services.WordPressPost{},
	}
}

func (w *fakeWordPress) id() int { w.nextID++; return w.nextID }

func (w *fakeWordPress) GetMedia(_ context.Context, id int) (*services.WordPressMedia, error) {
	m, ok := w.media[id]
	if !ok {
		return nil, fmt.Errorf("media %d not found", id)
	}
	return m, nil
}

func (w *fakeWordPress) Download(_ context.Context, url string) ([]byte, error) {
	return []byte("image:" + url), nil
}

// SearchMedia matches media whose title contains search (case-insensitive),
// like the WordPress ?search= parameter.
func (w *fakeWordPress) SearchMedia(_ context.Context, search string) ([]*services.WordPressMedia, error) {
	var out []*services.WordPressMedia
	for _, m := range w.media {
		if strings.Contains(strings.ToLower(m.Title.String()), strings.ToLower(search)) {
			out = append(out, m)
		}
	}
	return out, nil
}

func (w *fakeWordPress) UploadMediaFromFile(_ context.Context, filePath, title, altText string) (*services.WordPressMedia, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, err
	}
	id := w.id()
	m := &services.WordPressMedia{
		ID:        id,
		Title:     services.WordPressRenderedField{Rendered: title},
		AltText:   altText,
		SourceURL: "https://example.test/uploads/" + path.Base(filePath),
	}
	w.media[id] = m
	w.uploads = append(w.uploads, title)
	return m, nil
}

func (w *fakeWordPress) GetCategoryIDsByNames(_ context.Context, _ string, names []string) ([]int, error) {
	ids := make([]int, len(names))
	for i := range names {
		ids[i] = i + 1
	}
	return ids, nil
}

func (w *fakeWordPress) CreatePost(_ context.Context, post *services.WordPressPost) (*services.WordPressPost, error) {
	w.creates++
	created := *post
	created.ID = w.id()
	w.posts[created.ID] = &created
	return &created, nil
}

func (w *fakeWordPress) UpdatePost(_ context.Context, postID int, post *services.WordPressPost) (*services.WordPressPost, error) {
	if _, ok := w.posts[postID]; !ok {
		return nil, fmt.Errorf("post %d not found", postID)
	}
	w.updates = append(w.updates, postID)
	updated := *post
	updated.ID = postID
	w.posts[postID] = &updated
	return &updated, nil
}

var (
	_ MetadataStore   = (*fakeStore)(nil)
	_ DriveClient     = (*fakeDrive)(nil)
	_ ImageResizer    = (*fakeResizer)(nil)
	_ WordPressClient = (*fakeWordPress)(nil)
)
