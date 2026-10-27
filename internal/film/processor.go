package film

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"excentrico-tools-go/internal/drive"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/services"
	"excentrico-tools-go/internal/utils"
	"excentrico-tools-go/internal/wordpress"
)

// Processor handles the processing of individual films
type Processor struct {
	driveService        *services.GoogleDriveService
	imageService        *services.ImageService
	wordpressService    *services.WordPressService
	diviTemplateService *services.DiviTemplateService
	tursoService        *services.TursoService
}

// NewProcessor creates a new film processor with the required services
func NewProcessor(
	driveService *services.GoogleDriveService,
	imageService *services.ImageService,
	wordpressService *services.WordPressService,
	diviTemplateService *services.DiviTemplateService,
	tursoService *services.TursoService,
) *Processor {
	return &Processor{
		driveService:        driveService,
		imageService:        imageService,
		wordpressService:    wordpressService,
		diviTemplateService: diviTemplateService,
		tursoService:        tursoService,
	}
}

// ProcessSingleFilm processes a single film from the Google Sheet data.
// caption is written to <filmDir>/metadata.json as {"caption": ...}.
func (p *Processor) ProcessSingleFilm(ctx context.Context, obj map[string]any, baseDir string, year string, filmName string, caption string, templateConfig services.TemplateData) error {
	l := logger.Get()
	op := l.StartOperation("process_single_film")
	
	filmID := utils.SanitizeFilename(filmName)
	filmSection := utils.CellString(obj["SECCIÓN"])
	
	op.WithFilm(filmID, filmName, year, filmSection)
	
	sanitizedName := utils.SanitizeFilename(filmName)
	filmDir := filepath.Join(baseDir, sanitizedName)
	op.WithContext("film_dir", filmDir)

	if err := os.MkdirAll(filmDir, 0755); err != nil {
		op.Fail("Failed to create directory", err)
		return fmt.Errorf("failed to create directory: %v", err)
	}

	if err := WriteCaption(filmDir, caption); err != nil {
		op.Fail("Failed to write film caption", err)
		return err
	}

	// Process Google Drive files if available
	if enlaces, exists := obj["ENLACES"]; exists && enlaces != nil {
		enlacesStr := utils.CellString(enlaces)
		if enlacesStr != "" {
			driveOp := l.StartOperation("process_drive_files")
			driveOp.WithFilm(filmID, filmName, year, filmSection)
			driveOp.WithContext("enlaces_url", enlacesStr)
			
			if err := drive.ProcessGoogleDriveFiles(ctx, filmDir, p.driveService, p.imageService, p.tursoService, filmName, enlacesStr); err != nil {
				driveOp.Fail("Failed to process Google Drive files", err)
				return fmt.Errorf("failed to process Google Drive files: %v", err)
			}
			driveOp.Complete("Successfully processed Google Drive files")
		} else {
			driveOp := l.StartOperation("process_drive_files")
			driveOp.WithFilm(filmID, filmName, year, filmSection)
			driveOp.Warn(&logger.WideEvent{
				Message: fmt.Sprintf("No ENLACES URL found for film '%s'", filmName),
			})
		}
	} else {
		driveOp := l.StartOperation("process_drive_files")
		driveOp.WithFilm(filmID, filmName, year, filmSection)
		driveOp.Warn(&logger.WideEvent{
			Message: fmt.Sprintf("No ENLACES property found for film '%s'", filmName),
		})
	}

	// Upload media to WordPress
	wpOp := l.StartOperation("upload_wordpress_media")
	wpOp.WithFilm(filmID, filmName, year, filmSection)
	
	imageIds, err := wordpress.UploadMediaToWordPress(ctx, p.wordpressService, p.tursoService, filmDir, filmName, obj)
	if err != nil {
		wpOp.Fail("Failed to upload media to WordPress", err)
		return fmt.Errorf("failed to upload media to WordPress: %v", err)
	}
	wpOp.WithContext("image_count", len(imageIds))
	wpOp.Complete(fmt.Sprintf("Successfully uploaded %d images to WordPress", len(imageIds)))

	// Create or update WordPress project
	projectOp := l.StartOperation("create_update_wordpress_project")
	projectOp.WithFilm(filmID, filmName, year, filmSection)
	projectOp.WithContext("image_count", len(imageIds))
	
	if err := wordpress.CreateOrUpdateWordPressProject(ctx, p.wordpressService, p.diviTemplateService, p.tursoService, filmDir, obj, year, imageIds, templateConfig); err != nil {
		projectOp.Fail("Failed to create/update WordPress project", err)
		return fmt.Errorf("failed to create/update WordPress project: %v", err)
	}
	projectOp.Complete("Successfully created/updated WordPress project")

	op.Complete(fmt.Sprintf("Successfully processed film '%s'", filmName))
	return nil
}

// FilmMetadata is the content of films/<film>/metadata.json.
type FilmMetadata struct {
	Caption string `json:"caption"`
}

// WriteCaption writes {"caption": caption} to <filmDir>/metadata.json.
func WriteCaption(filmDir, caption string) error {
	data, err := json.MarshalIndent(FilmMetadata{Caption: caption}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode film metadata: %w", err)
	}
	path := filepath.Join(filmDir, "metadata.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
