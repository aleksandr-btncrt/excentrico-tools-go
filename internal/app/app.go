package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/film"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/models"
	"excentrico-tools-go/internal/services"

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

	// Initialize Google Sheets service
	sheetsService, err := services.NewGoogleSheetsService(ctx, cfg.GoogleCredentialsPath)
	if err != nil {
		return nil, err
	}

	// Initialize Google Drive service
	driveService, err := services.NewGoogleDriveService(ctx, cfg.GoogleCredentialsPath)
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
func (a *App) ProcessFilms(year string, templateConfig *services.TemplateData, metadata *models.Metadata) error {
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
			headers = append(headers, cell.(string))
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
				edicionStr := ""
				if edicion != nil {
					edicionStr = edicion.(string)
				}

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
func (a *App) processFilteredObjects(filteredObjects []map[string]any, year string, templateConfig *services.TemplateData, metadata *models.Metadata) error {
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
			filmName = name.(string)
		} else if name, exists := obj["Name"]; exists && name != nil {
			filmName = name.(string)
		}
		if name, exists := obj["SECCIÓN"]; exists && name != nil {
			filmSeccion = name.(string)
		}
		if name, exists := obj["DIRECCIÓN"]; exists && name != nil {
			filmDirect = name.(string)
		}

		filmID := strings.ToLower(strings.ReplaceAll(filmName, " ", "_"))
		SaveMetadata(filmName, CreateMetadata(filmName, filmSeccion, filmDirect, metadata, year))

		filmOp := l.StartOperation("process_single_film")
		filmOp.WithFilm(filmID, filmName, year, filmSeccion)
		filmOp.WithContext("film_index", processedCount)
		filmOp.WithContext("total_films", len(filteredObjects))

		if err := a.filmProcessor.ProcessSingleFilm(obj, baseDir, year, filmName, templateConfig); err != nil {
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

func CreateMetadata(movieName string, seccion string, direccion string , metadata *models.Metadata, year string) string {
	section :=  strings.ToUpper(seccion);
	
	// Handle nil metadata
	if metadata == nil {
		return section + " " + year + " - " + movieName + " - " + strings.ToUpper(strings.Replace(direccion, "y", "&", -1)) + " - " + "Programación Excéntrico " + year
	}
	
	// Check if Cities and Dates arrays have elements
	city := ""
	if len(metadata.Cities) > 0 {
		city = metadata.Cities[0]
	}
	
	dateFrom := ""
	dateTo := ""
	if len(metadata.Dates) > 0 && len(metadata.Dates[0]) > 0 {
		dateFrom = parseDate(metadata.Dates[0][0])
		if len(metadata.Dates[0]) > 1 {
			dateTo = parseDate(metadata.Dates[0][1])
		}
	}
	
	// Build the metadata string
	result := section + " " + year + " - " + movieName + " - " + strings.ToUpper(strings.Replace(direccion, "y", "&", -1)) + " - " + "Programación Excéntrico " + year
	if city != "" {
		result += " " + city
	}
	if dateFrom != "" {
		result += " del " + dateFrom
		if dateTo != "" {
			result += " al " + dateTo
		}
	}
	
	return result
}

func parseDate(date string) string {
	dateParts := strings.Split(date, "-")
	year, err := strconv.Atoi(dateParts[0])
	if err != nil {
		fmt.Println("Could not parse year:", err)
	}
	month, err := strconv.Atoi(dateParts[1])
	if err != nil {
		fmt.Println("Could not parse month:", err)
	}
	day, err := strconv.Atoi(dateParts[2])
	if err != nil {
		fmt.Println("Could not parse day:", err)
	}
	formatedDay := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	return monday.Format(formatedDay, "Monday 02 de January", monday.LocaleEsES)
}


func SaveMetadata(movieName string, metadata string)  {
	path := filepath.Join("films", movieName, "metadata.json")
	os.WriteFile(path, []byte(metadata), 0644)
}