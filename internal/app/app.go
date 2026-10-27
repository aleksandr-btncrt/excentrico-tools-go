package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/film"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/models"
	"excentrico-tools-go/internal/services"
	"excentrico-tools-go/internal/utils"

	"github.com/goodsign/monday"
)

// App encapsulates the application dependencies and configuration
type App struct {
	config              *config.Config
	sheetsService       *services.GoogleSheetsService
	driveService        *services.GoogleDriveService
	wordpressService    *services.WordPressService
	diviTemplateService *services.DiviTemplateService
	tursoService        *services.TursoService
	imageService        *services.ImageService
	filmProcessor       *film.Processor
}

// New creates a new application instance with all required services
func New(cfg *config.Config) (*App, error) {
	ctx := context.Background()

	// Initialize Google Drive service
	driveService, err := services.NewGoogleDriveService(ctx, cfg.GoogleCredentialsPath)
	if err != nil {
		return nil, err
	}

	// Initialize Google Sheets service. The Drive service is passed in so
	// ReadRange can fall back to downloading and parsing the file locally
	// when the configured Google Sheet ID actually points to a .xlsx file
	// (the Sheets API cannot read values from those - see issue #1).
	sheetsService, err := services.NewGoogleSheetsService(ctx, cfg.GoogleCredentialsPath, driveService)
	if err != nil {
		return nil, err
	}

	// Initialize WordPress service
	wordpressService := services.NewWordPressService(cfg.WordPressConfig)

	// Initialize Divi Template service
	diviTemplateService := services.NewDiviTemplateService()

	// Initialize Turso service
	tursoService, err := services.NewTursoService(cfg.TursoConfig)
	if err != nil {
		return nil, err
	}

	// Initialize Image service
	imageService := services.NewImageServiceWithConfig(
		cfg.ImageConfig.MaxWidth,
		cfg.ImageConfig.MaxHeight,
		cfg.ImageConfig.Quality,
	)

	// Initialize Film processor
	filmProcessor := film.NewProcessor(
		driveService,
		imageService,
		wordpressService,
		diviTemplateService,
		tursoService,
	)

	return &App{
		config:              cfg,
		sheetsService:       sheetsService,
		driveService:        driveService,
		wordpressService:    wordpressService,
		diviTemplateService: diviTemplateService,
		tursoService:        tursoService,
		imageService:        imageService,
		filmProcessor:       filmProcessor,
	}, nil
}

// Close cleans up resources
func (a *App) Close() {
	if a.tursoService != nil {
		a.tursoService.Close()
	}
}

// ListWordPressMenus fetches available WordPress navigation menus
func (a *App) ListWordPressMenus() ([]*services.WordPressMenu, error) {
	if a.wordpressService == nil {
		return nil, fmt.Errorf("wordpress service not initialized")
	}
	return a.wordpressService.GetNavMenus()
}

// ProcessFilms processes films from the Google Sheet with optional year filtering
func (a *App) ProcessFilms(year string, templateConfig services.TemplateData, metadata *models.Metadata) error {
	l := logger.Get()
	op := l.StartOperation("process_films")
	op.WithContext("year", year)
	op.WithContext("google_sheet_id", a.config.GoogleSheetID)
	
	if a.config.GoogleSheetID == "" {
		op.Fail("Google Sheet ID not configured", fmt.Errorf("please add 'google_sheet_id' to your configuration.json file"))
		log.Fatal("Google Sheet ID is not configured. Please add 'google_sheet_id' to your configuration.json file.")
	}

	data, err := a.sheetsService.ReadRange(a.config.GoogleSheetID, "TODO!A:ZZ")
	if err != nil {
		op.Fail("Failed to read data from Google Sheet", err)
		return err
	}

	if len(data) == 0 {
		op.Warn(&logger.WideEvent{
			Message: "No data found in the sheet",
		})
		return nil
	}

	if len(data) < 2 {
		op.Warn(&logger.WideEvent{
			Message: "Sheet must have at least 2 rows (headers + data)",
		})
		return nil
	}

	// Extract headers
	headers := make([]string, 0)
	for _, cell := range data[0] {
		if cell != nil {
			headers = append(headers, utils.CellString(cell))
		}
	}

	op.WithContext("header_count", len(headers))
	op.WithContext("data_row_count", len(data)-1)

	// Convert rows to objects and apply filtering
	objects := make([]map[string]any, 0)
	filteredObjects := make([]map[string]any, 0)
	matchedCount := 0
	excludedCount := 0

	for i := 1; i < len(data); i++ {
		row := data[i]
		obj := make(map[string]any)

		for j, header := range headers {
			if j < len(row) && row[j] != nil {
				obj[header] = row[j]
			} else {
				obj[header] = ""
			}
		}

		objects = append(objects, obj)

		// Apply year filter if specified
		if year != "" {
			if edicion, exists := obj["EDICIÓN"]; exists {
				edicionStr := utils.CellString(edicion)

				expectedEdicion := "Excéntrico " + year
				if strings.EqualFold(edicionStr, expectedEdicion) {
					filteredObjects = append(filteredObjects, obj)
					matchedCount++
				} else {
					excludedCount++
				}
			} else {
				excludedCount++
			}
		} else {
			filteredObjects = append(filteredObjects, obj)
			matchedCount++
		}
	}

	op.WithContext("total_objects", len(objects))
	op.WithContext("filtered_objects", len(filteredObjects))
	op.WithContext("matched_count", matchedCount)
	op.WithContext("excluded_count", excludedCount)
	
	if year != "" {
		op.WithContext("year_filter", year)
	}

	if len(filteredObjects) > 0 {
		err := a.processFilteredObjects(filteredObjects, year, templateConfig, metadata)
		if err != nil {
			op.Fail("Failed to process filtered objects", err)
			return err
		}
		op.Complete(fmt.Sprintf("Successfully processed %d films", len(filteredObjects)))
		return nil
	}

	op.Complete("Sheet processing completed - no films to process")
	return nil
}

// processFilteredObjects processes the filtered film objects
func (a *App) processFilteredObjects(filteredObjects []map[string]any, year string, templateConfig services.TemplateData, metadata *models.Metadata) error {
	l := logger.Get()
	op := l.StartOperation("process_filtered_objects")
	op.WithContext("total_films", len(filteredObjects))
	op.WithContext("year", year)

	baseDir := "films"
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		op.Fail("Failed to create films directory", err)
		return err
	}

	var processedCount, successCount, errorCount int

	for _, obj := range filteredObjects {
		processedCount++

		filmName := "unnamed_film"
		filmSeccion := "unnamed_section"
		filmDirect := "unammed_directs"
		if name, exists := obj["TÍTULO ORIGINAL"]; exists && name != nil {
			filmName = utils.CellString(name)
		} else if name, exists := obj["Name"]; exists && name != nil {
			filmName = utils.CellString(name)
		}
		if name, exists := obj["SECCIÓN"]; exists && name != nil {
			filmSeccion = utils.CellString(name)
		}
		if name, exists := obj["DIRECCIÓN"]; exists && name != nil {
			filmDirect = utils.CellString(name)
		}

		filmID := strings.ToLower(strings.ReplaceAll(filmName, " ", "_"))

		filmOp := l.StartOperation("process_single_film")
		filmOp.WithFilm(filmID, filmName, year, filmSeccion)
		filmOp.WithContext("film_index", processedCount)
		filmOp.WithContext("total_films", len(filteredObjects))

		caption, err := CreateMetadata(filmName, filmSeccion, filmDirect, metadata, year)
		if err != nil {
			filmOp.Fail(fmt.Sprintf("Failed to build caption for film '%s'", filmName), err)
			errorCount++
			continue
		}

		if err := a.filmProcessor.ProcessSingleFilm(obj, baseDir, year, filmName, caption, templateConfig); err != nil {
			filmOp.Fail(fmt.Sprintf("Failed to process film '%s'", filmName), err)
			errorCount++
		} else {
			filmOp.Complete(fmt.Sprintf("Successfully processed film '%s'", filmName))
			successCount++
		}
	}

	op.WithCounts(processedCount, 0, 0, 0, 0, 0)
	op.WithContext("success_count", successCount)
	op.WithContext("error_count", errorCount)
	op.Complete(fmt.Sprintf("Processing completed: %d total, %d successful, %d failed", processedCount, successCount, errorCount))

	return nil
}

// directorSplitRe splits a multi-director cell ("A, B y C", "A & B", "A + B").
// Temporary copy of the regex in wordpress/operations.go; de-duplicated when
// the sheet package is introduced (plan Step 5).
var directorSplitRe = regexp.MustCompile(`\s*,\s*|\s+\+\s+|\s+y\s+|\s*&\s*`)

// formatDirectors splits the director cell into names and joins them with
// " & " in upper case. Only whole-word separators are replaced, so names that
// contain the letter "y" keep it (previously "Maya" became "MA&A").
func formatDirectors(direccion string) string {
	var names []string
	for _, p := range directorSplitRe.Split(direccion, -1) {
		if p = strings.TrimSpace(p); p != "" {
			names = append(names, p)
		}
	}
	return strings.ToUpper(strings.Join(names, " & "))
}

// CreateMetadata builds the film caption written to films/<film>/metadata.json:
//
//	SECCIÓN YEAR - Title - DIRECTOR & DIRECTOR - Programación Excéntrico YEAR [City] [del <date> [al <date>]]
//
// It returns an error when a date in metadata is not in YYYY-MM-DD format.
func CreateMetadata(movieName string, seccion string, direccion string, metadata *models.Metadata, year string) (string, error) {
	result := strings.ToUpper(seccion) + " " + year + " - " + movieName + " - " + formatDirectors(direccion) + " - " + "Programación Excéntrico " + year
	if metadata == nil {
		return result, nil
	}

	if len(metadata.Cities) > 0 && metadata.Cities[0] != "" {
		result += " " + metadata.Cities[0]
	}

	if len(metadata.Dates) > 0 && len(metadata.Dates[0]) > 0 {
		dateFrom, err := parseDate(metadata.Dates[0][0])
		if err != nil {
			return "", err
		}
		result += " del " + dateFrom
		if len(metadata.Dates[0]) > 1 {
			dateTo, err := parseDate(metadata.Dates[0][1])
			if err != nil {
				return "", err
			}
			result += " al " + dateTo
		}
	}

	return result, nil
}

// parseDate formats a YYYY-MM-DD date as a Spanish long date, e.g.
// "2026-01-23" -> "viernes 23 de enero".
func parseDate(date string) (string, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(date))
	if err != nil {
		return "", fmt.Errorf("invalid metadata date %q (want YYYY-MM-DD): %w", date, err)
	}
	return monday.Format(t, "Monday 02 de January", monday.LocaleEsES), nil
}
