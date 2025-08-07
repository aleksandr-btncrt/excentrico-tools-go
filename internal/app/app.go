package app

import (
	"context"
	"log"
	"os"
	"strings"

	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/film"
	"excentrico-tools-go/internal/services"
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

// ProcessFilms processes films from the Google Sheet with optional year filtering
func (a *App) ProcessFilms(year string) error {
	if a.config.GoogleSheetID == "" {
		log.Fatal("Google Sheet ID is not configured. Please add 'google_sheet_id' to your configuration.json file.")
	}

	log.Println("Reading data from Google Sheet...")

	data, err := a.sheetsService.ReadRange(a.config.GoogleSheetID, "TODO!A:ZZ")
	if err != nil {
		return err
	}

	if len(data) == 0 {
		log.Println("No data found in the sheet")
		return nil
	}

	if len(data) < 2 {
		log.Println("Sheet must have at least 2 rows (headers + data)")
		return nil
	}

	// Extract headers
	headers := make([]string, 0)
	for _, cell := range data[0] {
		if cell != nil {
			headers = append(headers, cell.(string))
		}
	}

	log.Printf("Found %d headers: %v", len(headers), headers)
	log.Printf("Found %d data rows", len(data)-1)
	log.Println("================")

	// Convert rows to objects and apply filtering
	objects := make([]map[string]any, 0)
	filteredObjects := make([]map[string]any, 0)

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
					log.Printf("✅ MATCHES FILTER - Object %d included", i)
				} else {
					log.Printf("❌ DOES NOT MATCH - Object %d excluded (has '%s', expected '%s')", i, edicionStr, expectedEdicion)
				}
			} else {
				log.Printf("❌ NO EDICIÓN FIELD - Object %d excluded", i)
			}
		} else {
			filteredObjects = append(filteredObjects, obj)
			log.Printf("✅ NO FILTER - Object %d included", i)
		}
	}

	log.Println("================")
	log.Printf("Successfully transformed %d objects from sheet", len(objects))

	if year != "" {
		log.Printf("Filtered by year '%s': %d objects match", year, len(filteredObjects))
	} else {
		log.Printf("No year filter applied: showing all %d objects", len(filteredObjects))
	}

	if len(filteredObjects) > 0 {
		return a.processFilteredObjects(filteredObjects, year)
	}

	log.Println("Sheet processing completed")
	return nil
}

// processFilteredObjects processes the filtered film objects
func (a *App) processFilteredObjects(filteredObjects []map[string]any, year string) error {
	log.Println("================")
	log.Println("Processing filtered objects...")

	baseDir := "films"
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		log.Printf("Failed to create films directory: %v", err)
		return err
	}

	var processedCount, successCount, errorCount int

	for _, obj := range filteredObjects {
		processedCount++

		filmName := "unnamed_film"
		if name, exists := obj["TÍTULO ORIGINAL"]; exists && name != nil {
			filmName = name.(string)
		} else if name, exists := obj["Name"]; exists && name != nil {
			filmName = name.(string)
		}

		log.Printf("Processing film %d/%d: %s", processedCount, len(filteredObjects), filmName)

		if err := a.filmProcessor.ProcessSingleFilm(obj, baseDir, year, filmName); err != nil {
			log.Printf("❌ Failed to process film '%s': %v", filmName, err)
			errorCount++
		} else {
			log.Printf("✅ Successfully processed film '%s'", filmName)
			successCount++
		}
	}

	log.Println("================")
	log.Printf("Processing Summary:")
	log.Printf("Total films: %d", processedCount)
	log.Printf("Successful: %d", successCount)
	log.Printf("Failed: %d", errorCount)
	log.Println("================")

	return nil
}
