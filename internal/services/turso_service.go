package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"excentrico-tools-go/internal/config"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

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

	log.Printf("Successfully connected to Turso database")
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

	log.Printf("Saved metadata for film '%s' (type: %s)", filmID, metadataType)
	return nil
}

func (s *TursoService) GetMetadata(filmID, metadataType string, dest interface{}) error {
	query := `SELECT data FROM metadata WHERE film_id = ? AND type = ?`

	var jsonData string
	err := s.db.QueryRow(query, filmID, metadataType).Scan(&jsonData)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("metadata not found for film '%s' type '%s'", filmID, metadataType)
		}
		return fmt.Errorf("failed to get metadata: %v", err)
	}

	if err := json.Unmarshal([]byte(jsonData), dest); err != nil {
		return fmt.Errorf("failed to unmarshal JSON data: %v", err)
	}

	return nil
}

func (s *TursoService) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *TursoService) SaveWordPressMetadata(filmID string, metadata interface{}) error {
	return s.SaveMetadata(filmID, "wordpress", metadata)
}

func (s *TursoService) GetWordPressMetadata(filmID string, dest interface{}) error {
	return s.GetMetadata(filmID, "wordpress", dest)
}

func (s *TursoService) SaveDriveFilesMetadata(filmID string, files interface{}) error {
	return s.SaveMetadata(filmID, "drive_files", files)
}

func (s *TursoService) GetDriveFilesMetadata(filmID string, dest interface{}) error {
	return s.GetMetadata(filmID, "drive_files", dest)
}

func (s *TursoService) SaveWPImagesMetadata(filmID string, images interface{}) error {
	return s.SaveMetadata(filmID, "wp_images", images)
}

func (s *TursoService) GetWPImagesMetadata(filmID string, dest interface{}) error {
	return s.GetMetadata(filmID, "wp_images", dest)
}
