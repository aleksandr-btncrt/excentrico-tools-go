package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/film"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/models"
	"excentrico-tools-go/internal/services"
	"excentrico-tools-go/internal/sheet"
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

// SetEmbedImages controls whether divi_template.json embeds each image as
// base64 (-embed-images).
func (a *App) SetEmbedImages(embed bool) {
	a.diviTemplateService.EmbedImages = embed
}

// Close cleans up resources
func (a *App) Close() {
	if a.tursoService != nil {
		a.tursoService.Close()
	}
}

// ListWordPressMenus fetches available WordPress navigation menus
func (a *App) ListWordPressMenus(ctx context.Context) ([]*services.WordPressMenu, error) {
	if a.wordpressService == nil {
		return nil, fmt.Errorf("wordpress service not initialized")
	}
	return a.wordpressService.GetNavMenus(ctx)
}

// ProcessOptions selects and controls a processing run.
type ProcessOptions struct {
	// Year keeps only rows whose EDICIÓN is edition_prefix + Year, e.g.
	// "Excéntrico 2026" (all rows when empty).
	Year string
	// Film keeps only the row whose TÍTULO ORIGINAL matches (case-insensitive).
	Film string
	// DryRun reads and filters the sheet and reports what would be processed,
	// without downloading, uploading or writing anything.
	DryRun   bool
	Template services.TemplateData
	Metadata *models.Metadata
}

// sheetOptions returns the sheet layout from the configuration (defaults
// when unset).
func (a *App) sheetOptions() sheet.Options {
	o := sheet.DefaultOptions()
	if a.config != nil {
		if a.config.SheetTab != "" {
			o.Tab = a.config.SheetTab
		}
		if a.config.EditionPrefix != "" {
			o.EditionPrefix = a.config.EditionPrefix
		}
	}
	return o
}

// ProcessFilms reads the Google Sheet, selects the films and processes each
// one, printing one progress line per film.
func (a *App) ProcessFilms(ctx context.Context, opts ProcessOptions) error {
	l := logger.Get()
	sheetOpts := a.sheetOptions()
	op := l.StartOperation("read_sheet")
	op.WithContext("year", opts.Year)
	op.WithContext("google_sheet_id", a.config.GoogleSheetID)
	op.WithContext("sheet_range", sheetOpts.DataRange())

	if a.config.GoogleSheetID == "" {
		err := errors.New("google_sheet_id is not configured; add it to configuration.json")
		op.Fail("Google Sheet ID not configured", err)
		return err
	}

	data, err := a.sheetsService.ReadRange(ctx, a.config.GoogleSheetID, sheetOpts.DataRange())
	if err != nil {
		op.Fail("Failed to read data from Google Sheet", err)
		return err
	}
	all := sheet.Films(data)
	if len(all) == 0 {
		op.Warn(&logger.WideEvent{Message: "Sheet must have at least 2 rows (headers + data)"})
		l.Progress("The sheet has no data rows; nothing to process.")
		return nil
	}

	films := sheetOpts.Select(all, opts.Year, opts.Film)
	op.WithContext("data_row_count", len(all))
	op.WithContext("selected_count", len(films))
	op.WithContext("film_filter", opts.Film)
	op.Complete(fmt.Sprintf("Read %d rows, %d selected", len(all), len(films)))

	if len(films) == 0 {
		l.Progress("No films matched (edition %q, film %q).", sheetOpts.Edition(opts.Year), opts.Film)
		return nil
	}
	return a.processFilms(ctx, films, opts)
}

// processFilms processes the selected films in order. A failing film is
// reported and skipped; an interrupt stops before the next film.
func (a *App) processFilms(ctx context.Context, films []*sheet.Film, opts ProcessOptions) error {
	l := logger.Get()
	op := l.StartOperation("process_films").Important()
	op.WithContext("total_films", len(films))
	op.WithContext("year", opts.Year)
	op.WithContext("dry_run", opts.DryRun)

	baseDir := "films"
	if !opts.DryRun {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			op.Fail("Failed to create films directory", err)
			return err
		}
	}

	total := len(films)
	var processed, succeeded, failed int
	for i, f := range films {
		if err := ctx.Err(); err != nil {
			op.Fail(fmt.Sprintf("Processing interrupted after %d of %d films", processed, total), err)
			l.Progress("Interrupted after %d of %d films.", processed, total)
			return err
		}
		processed++

		title := f.Title()
		prefix := fmt.Sprintf("[%d/%d] %q", i+1, total, title)

		caption, err := CreateMetadata(title, f.Seccion, f.Direccion, opts.Metadata, opts.Year)
		if err != nil {
			l.StartOperation("build_caption").WithFilm(utils.SanitizeFilename(title), title, opts.Year, f.Seccion).Fail("Failed to build caption", err)
			l.Progress("%s → FAILED: %v", prefix, err)
			failed++
			continue
		}

		if opts.DryRun {
			l.Progress("%s → would process (section %q, directors %q)", prefix, f.Seccion, f.Direccion)
			succeeded++
			continue
		}

		res, err := a.filmProcessor.ProcessSingleFilm(ctx, f, baseDir, opts.Year, caption, opts.Template)
		if err != nil {
			l.Progress("%s → FAILED: %v", prefix, err)
			failed++
			continue
		}
		l.Progress("%s → post #%d, %d images", prefix, res.PostID, res.Images)
		succeeded++
	}

	op.WithContext("success_count", succeeded)
	op.WithContext("error_count", failed)
	op.Complete(fmt.Sprintf("Processing completed: %d total, %d successful, %d failed", processed, succeeded, failed))
	l.Progress("Done: %d films, %d succeeded, %d failed.", processed, succeeded, failed)
	return nil
}

// CheckResult is one line of the -check report.
type CheckResult struct {
	Name   string
	Detail string
	Err    error
}

// Check verifies WordPress authentication, Turso connectivity and that the
// sheet's header row can be read. It never writes anything.
func (a *App) Check(ctx context.Context) []CheckResult {
	var results []CheckResult

	user, err := a.wordpressService.TestConnection(ctx)
	results = append(results, CheckResult{Name: "WordPress", Detail: "authenticated as " + user, Err: err})

	err = a.tursoService.Ping(ctx)
	results = append(results, CheckResult{Name: "Turso", Detail: "connected", Err: err})

	data, err := a.sheetsService.ReadRange(ctx, a.config.GoogleSheetID, a.sheetOptions().HeaderRange())
	detail := ""
	if err == nil {
		if len(data) == 0 {
			err = errors.New("header row is empty")
		} else {
			detail = fmt.Sprintf("%d header columns", len(data[0]))
		}
	}
	results = append(results, CheckResult{Name: "Google Sheet", Detail: detail, Err: err})

	return results
}

// formatDirectors splits the director cell into names and joins them with
// " & " in upper case. Only whole-word separators are replaced, so names that
// contain the letter "y" keep it (previously "Maya" became "MA&A").
func formatDirectors(direccion string) string {
	return strings.ToUpper(strings.Join(sheet.SplitDirectors(direccion), " & "))
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
