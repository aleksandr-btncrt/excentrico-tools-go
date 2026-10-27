package app

import (
	"context"
	"errors"
	"fmt"
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
func (a *App) ListWordPressMenus(ctx context.Context) ([]*services.WordPressMenu, error) {
	if a.wordpressService == nil {
		return nil, fmt.Errorf("wordpress service not initialized")
	}
	return a.wordpressService.GetNavMenus(ctx)
}

// ProcessOptions selects and controls a processing run.
type ProcessOptions struct {
	// Year keeps only rows whose EDICIÓN is "Excéntrico <Year>" (all rows
	// when empty).
	Year string
	// Film keeps only the row whose TÍTULO ORIGINAL matches (case-insensitive).
	Film string
	// DryRun reads and filters the sheet and reports what would be processed,
	// without downloading, uploading or writing anything.
	DryRun   bool
	Template services.TemplateData
	Metadata *models.Metadata
}

// ProcessFilms reads the Google Sheet, filters the rows and processes each
// selected film, printing one progress line per film.
func (a *App) ProcessFilms(ctx context.Context, opts ProcessOptions) error {
	l := logger.Get()
	op := l.StartOperation("read_sheet")
	op.WithContext("year", opts.Year)
	op.WithContext("google_sheet_id", a.config.GoogleSheetID)

	if a.config.GoogleSheetID == "" {
		err := errors.New("google_sheet_id is not configured; add it to configuration.json")
		op.Fail("Google Sheet ID not configured", err)
		return err
	}

	data, err := a.sheetsService.ReadRange(ctx, a.config.GoogleSheetID, "TODO!A:ZZ")
	if err != nil {
		op.Fail("Failed to read data from Google Sheet", err)
		return err
	}
	if len(data) < 2 {
		op.Warn(&logger.WideEvent{Message: "Sheet must have at least 2 rows (headers + data)"})
		l.Progress("The sheet has no data rows; nothing to process.")
		return nil
	}

	films := filterRows(rowsToObjects(data), opts.Year, opts.Film)
	op.WithContext("data_row_count", len(data)-1)
	op.WithContext("selected_count", len(films))
	op.WithContext("film_filter", opts.Film)
	op.Complete(fmt.Sprintf("Read %d rows, %d selected", len(data)-1, len(films)))

	if len(films) == 0 {
		l.Progress("No films matched (year %q, film %q).", opts.Year, opts.Film)
		return nil
	}
	return a.processFilteredObjects(ctx, films, opts)
}

// rowsToObjects maps each data row to header -> cell (missing cells are "").
func rowsToObjects(data [][]any) []map[string]any {
	headers := make([]string, 0, len(data[0]))
	for _, cell := range data[0] {
		headers = append(headers, utils.CellString(cell))
	}
	objects := make([]map[string]any, 0, len(data)-1)
	for _, row := range data[1:] {
		obj := make(map[string]any, len(headers))
		for j, header := range headers {
			if header == "" {
				continue
			}
			if j < len(row) && row[j] != nil {
				obj[header] = row[j]
			} else {
				obj[header] = ""
			}
		}
		objects = append(objects, obj)
	}
	return objects
}

// filterRows keeps rows for the edition year ("Excéntrico <year>") and, when
// film is set, the row whose TÍTULO ORIGINAL equals it (case-insensitive).
func filterRows(objects []map[string]any, year, film string) []map[string]any {
	film = strings.TrimSpace(film)
	var out []map[string]any
	for _, obj := range objects {
		if year != "" && !strings.EqualFold(strings.TrimSpace(utils.CellString(obj["EDICIÓN"])), "Excéntrico "+year) {
			continue
		}
		if film != "" && !strings.EqualFold(strings.TrimSpace(filmTitle(obj)), film) {
			continue
		}
		out = append(out, obj)
	}
	return out
}

// filmTitle returns the row's TÍTULO ORIGINAL (or Name), or "unnamed_film".
func filmTitle(obj map[string]any) string {
	if t := utils.CellString(obj["TÍTULO ORIGINAL"]); t != "" {
		return t
	}
	if t := utils.CellString(obj["Name"]); t != "" {
		return t
	}
	return "unnamed_film"
}

// processFilteredObjects processes the selected films in order. A failing
// film is reported and skipped; an interrupt stops before the next film.
func (a *App) processFilteredObjects(ctx context.Context, films []map[string]any, opts ProcessOptions) error {
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
	for i, obj := range films {
		if err := ctx.Err(); err != nil {
			op.Fail(fmt.Sprintf("Processing interrupted after %d of %d films", processed, total), err)
			l.Progress("Interrupted after %d of %d films.", processed, total)
			return err
		}
		processed++

		title := filmTitle(obj)
		section := utils.CellString(obj["SECCIÓN"])
		directors := utils.CellString(obj["DIRECCIÓN"])
		prefix := fmt.Sprintf("[%d/%d] %q", i+1, total, title)

		caption, err := CreateMetadata(title, section, directors, opts.Metadata, opts.Year)
		if err != nil {
			l.StartOperation("build_caption").WithFilm(utils.SanitizeFilename(title), title, opts.Year, section).Fail("Failed to build caption", err)
			l.Progress("%s → FAILED: %v", prefix, err)
			failed++
			continue
		}

		if opts.DryRun {
			l.Progress("%s → would process (section %q, directors %q)", prefix, section, directors)
			succeeded++
			continue
		}

		res, err := a.filmProcessor.ProcessSingleFilm(ctx, obj, baseDir, opts.Year, title, caption, opts.Template)
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

	data, err := a.sheetsService.ReadRange(ctx, a.config.GoogleSheetID, "TODO!1:1")
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
