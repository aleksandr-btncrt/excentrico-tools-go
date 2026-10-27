package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/debug"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

// ErrMetadataNotFound is returned (wrapped) by GetMetadata and its typed
// helpers when no row exists for the film/type. Check with errors.Is.
var ErrMetadataNotFound = errors.New("metadata not found")

type TursoService struct {
	db *sql.DB
}

func NewTursoService(cfg config.TursoConfig) (*TursoService, error) {

	db, err := sql.Open("libsql", cfg.DatabaseURL+"?authToken="+cfg.AuthToken)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Turso database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping Turso database: %v", err)
	}

	service := &TursoService{db: db}

	if err := service.initializeTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize database tables: %v", err)
	}

	return service, nil
}

func (s *TursoService) initializeTables() error {
	query := `
		CREATE TABLE IF NOT EXISTS metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			film_id TEXT NOT NULL,
			type TEXT NOT NULL,
			data TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE UNIQUE INDEX IF NOT EXISTS idx_film_type ON metadata(film_id, type);
		CREATE INDEX IF NOT EXISTS idx_film_id ON metadata(film_id);
		CREATE INDEX IF NOT EXISTS idx_type ON metadata(type);
	`

	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create tables: %v", err)
	}

	return nil
}

func (s *TursoService) SaveMetadata(filmID, metadataType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %v", err)
	}

	query := `
		INSERT INTO metadata (film_id, type, data, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(film_id, type) DO UPDATE SET
			data = excluded.data,
			updated_at = CURRENT_TIMESTAMP
	`

	if _, err := s.db.Exec(query, filmID, metadataType, string(jsonData)); err != nil {
		return fmt.Errorf("failed to save metadata: %v", err)
	}

	debug.Printf("Saved metadata for film '%s' (type: %s)", filmID, metadataType)
	return nil
}

func (s *TursoService) GetMetadata(filmID, metadataType string, dest interface{}) error {
	query := `SELECT data FROM metadata WHERE film_id = ? AND type = ?`

	var jsonData string
	err := s.db.QueryRow(query, filmID, metadataType).Scan(&jsonData)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w for film '%s' type '%s'", ErrMetadataNotFound, filmID, metadataType)
		}
		return fmt.Errorf("failed to get metadata: %v", err)
	}

	if err := json.Unmarshal([]byte(jsonData), dest); err != nil {
		return fmt.Errorf("failed to unmarshal JSON data: %v", err)
	}

	return nil
}

// Ping checks that the database is reachable.
func (s *TursoService) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("turso ping: %w", err)
	}
	return nil
}

func (s *TursoService) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// MetadataStore persists JSON metadata per film and type. TursoService is
// the production implementation; tests use an in-memory fake. GetMetadata
// must return an error wrapping ErrMetadataNotFound when nothing is stored.
type MetadataStore interface {
	SaveMetadata(filmID, metadataType string, data any) error
	GetMetadata(filmID, metadataType string, dest any) error
}

var _ MetadataStore = (*TursoService)(nil)

// Metadata types stored per film.
const (
	MetadataWordPress      = "wordpress"
	MetadataDriveFiles     = "drive_files"
	MetadataWPImages       = "wp_images"
	MetadataWordPressMedia = "wordpress_media"
	MetadataDirectorMedia  = "director_media"
)

// SaveWordPressMetadata stores the film's post metadata.
func SaveWordPressMetadata(store MetadataStore, filmID string, metadata any) error {
	return store.SaveMetadata(filmID, MetadataWordPress, metadata)
}

// GetWordPressMetadata loads the film's post metadata.
func GetWordPressMetadata(store MetadataStore, filmID string, dest any) error {
	return store.GetMetadata(filmID, MetadataWordPress, dest)
}

// SaveDriveFilesMetadata stores the film's Drive file list.
func SaveDriveFilesMetadata(store MetadataStore, filmID string, files any) error {
	return store.SaveMetadata(filmID, MetadataDriveFiles, files)
}

// GetDriveFilesMetadata loads the film's Drive file list.
func GetDriveFilesMetadata(store MetadataStore, filmID string, dest any) error {
	return store.GetMetadata(filmID, MetadataDriveFiles, dest)
}

// SaveWPImagesMetadata stores the film's file name -> media ID map.
func SaveWPImagesMetadata(store MetadataStore, filmID string, images any) error {
	return store.SaveMetadata(filmID, MetadataWPImages, images)
}

// GetWPImagesMetadata loads the film's file name -> media ID map.
func GetWPImagesMetadata(store MetadataStore, filmID string, dest any) error {
	return store.GetMetadata(filmID, MetadataWPImages, dest)
}

// DirectorMediaCacheKey is the global film_id used to store director image cache (shared across all films).
const DirectorMediaCacheKey = "__directors__"

// DirectorMediaCacheEntry holds WordPress media id and URL for a director image.
type DirectorMediaCacheEntry struct {
	ID        int    `json:"id"`
	SourceURL string `json:"source_url"`
}

// GetDirectorMediaCache loads the director name -> media cache shared by all films.
func GetDirectorMediaCache(store MetadataStore, dest any) error {
	return store.GetMetadata(DirectorMediaCacheKey, MetadataDirectorMedia, dest)
}

// SaveDirectorMediaCache stores the director name -> media cache shared by all films.
func SaveDirectorMediaCache(store MetadataStore, data any) error {
	return store.SaveMetadata(DirectorMediaCacheKey, MetadataDirectorMedia, data)
}
