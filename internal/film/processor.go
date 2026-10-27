package film

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"excentrico-tools-go/internal/drive"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/services"
	"excentrico-tools-go/internal/sheet"
	"excentrico-tools-go/internal/utils"
	"excentrico-tools-go/internal/wordpress"
)

// Processor runs the per-film pipeline: Drive download + resize, media
// upload, WordPress post and Divi template.
type Processor struct {
	drive     DriveClient
	images    ImageResizer
	wordpress WordPressClient
	divi      *services.DiviTemplateService
	store     MetadataStore
}

// NewProcessor creates a film processor over the given dependencies.
func NewProcessor(
	driveClient DriveClient,
	images ImageResizer,
	wp WordPressClient,
	divi *services.DiviTemplateService,
	store MetadataStore,
) *Processor {
	return &Processor{
		drive:     driveClient,
		images:    images,
		wordpress: wp,
		divi:      divi,
		store:     store,
	}
}

// Result summarises one processed film for the progress line.
type Result struct {
	PostID int
	Images int
}

// ProcessSingleFilm processes a single film from the Google Sheet data:
// Drive download + resize, media upload, WordPress post and Divi template.
// caption is written to <filmDir>/metadata.json as {"caption": ...}.
// It emits exactly one "process_single_film" event (never sampled away).
func (p *Processor) ProcessSingleFilm(ctx context.Context, f *sheet.Film, baseDir string, year string, caption string, templateConfig services.TemplateData) (Result, error) {
	filmName := f.Title()
	l := logger.Get()
	op := l.StartOperation("process_single_film").Important()

	filmID := utils.SanitizeFilename(filmName)
	filmSection := f.Seccion
	op.WithFilm(filmID, filmName, year, filmSection)

	filmDir := filepath.Join(baseDir, filmID)
	op.WithContext("film_dir", filmDir)

	fail := func(msg string, err error) (Result, error) {
		op.Fail(msg, err)
		return Result{}, fmt.Errorf("%s: %w", strings.ToLower(msg[:1])+msg[1:], err)
	}

	if err := os.MkdirAll(filmDir, 0755); err != nil {
		return fail("Failed to create directory", err)
	}

	if err := WriteCaption(filmDir, caption); err != nil {
		return fail("Failed to write film caption", err)
	}

	// Download and resize Google Drive files when the sheet links a folder.
	// drive.ProcessGoogleDriveFiles logs its own "process_drive_files" event.
	if strings.TrimSpace(f.Enlaces) != "" {
		if err := drive.ProcessGoogleDriveFiles(ctx, filmDir, p.drive, p.images, p.store, f); err != nil {
			return fail("Failed to process Google Drive files", err)
		}
	} else {
		op.WithContext("drive_skipped", "no ENLACES URL")
	}

	imageIds, err := wordpress.UploadMediaToWordPress(ctx, p.wordpress, p.store, filmDir, f)
	if err != nil {
		return fail("Failed to upload media to WordPress", err)
	}
	op.WithContext("image_count", len(imageIds))

	postID, err := wordpress.CreateOrUpdateWordPressProject(ctx, p.wordpress, p.divi, p.store, filmDir, f, year, imageIds, templateConfig)
	if err != nil {
		return fail("Failed to create/update WordPress project", err)
	}
	op.WithWordPress(postID, 0, "")

	op.Complete(fmt.Sprintf("Successfully processed film '%s'", filmName))
	return Result{PostID: postID, Images: len(imageIds)}, nil
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
