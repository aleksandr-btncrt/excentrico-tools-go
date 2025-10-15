package film

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"excentrico-tools-go/internal/drive"
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

// ProcessSingleFilm processes a single film from the Google Sheet data
func (p *Processor) ProcessSingleFilm(obj map[string]any, baseDir string, year string, filmName string, templateConfig *services.TemplateData) error {
	sanitizedName := utils.SanitizeFilename(filmName)
	filmDir := filepath.Join(baseDir, sanitizedName)

	if err := os.MkdirAll(filmDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	log.Printf("Created directory: %s", filmDir)

	// Process Google Drive files if available
	if enlaces, exists := obj["ENLACES"]; exists && enlaces != nil {
		enlacesStr := enlaces.(string)
		if enlacesStr != "" {
			if err := drive.ProcessGoogleDriveFiles(filmDir, p.driveService, p.imageService, p.tursoService, filmName, enlacesStr); err != nil {
				return fmt.Errorf("failed to process Google Drive files: %v", err)
			}
		} else {
			log.Printf("No ENLACES URL found for film '%s'", filmName)
		}
	} else {
		log.Printf("No ENLACES property found for film '%s'", filmName)
	}

	// Upload media to WordPress
	log.Printf("Uploading media to WordPress for '%s'", filmName)
	imageIds, err := wordpress.UploadMediaToWordPress(p.wordpressService, p.tursoService, filmDir, filmName)
	if err != nil {
		return fmt.Errorf("failed to upload media to WordPress: %v", err)
	}

	// Create or update WordPress project
	log.Printf("Creating/updating WordPress project for '%s'", filmName)
	if err := wordpress.CreateOrUpdateWordPressProject(p.wordpressService, p.diviTemplateService, p.tursoService, filmDir, obj, year, imageIds, templateConfig); err != nil {
		return fmt.Errorf("failed to create/update WordPress project: %v", err)
	}

	return nil
}
