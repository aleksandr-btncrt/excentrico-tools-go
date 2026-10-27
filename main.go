package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"excentrico-tools-go/internal/app"
	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/debug"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/models"
	"excentrico-tools-go/internal/services"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// stdin is shared by all prompts so buffered input is not lost between them.
var stdin = bufio.NewScanner(os.Stdin)

type RuntimeOptions struct {
	Menu         string
	Year         string
	TemplatePath string
	// MenuSlug is the WordPress navigation menu slug (-menu-slug / -nav-menu).
	MenuSlug       string
	Film           string
	NonInteractive bool
	DryRun         bool
	Check          bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run is the whole program; every failure is returned here so main has a
// single exit path (deferred cleanup always runs).
func run() error {
	createConfig := flag.Bool("create-config", false, "Create a default configuration file")
	yearFlag := flag.String("year", "", "Edition year; selects templates/<year>.json and metadata/<year>.json")
	debugFlag := flag.Bool("debug", false, "Write debug events to the log file")
	menuFlag := flag.String("menu", "", "Action to run: configuration | process")
	menuSlugFlag := flag.String("menu-slug", "", "WordPress navigation menu slug; its ID overrides menu.menu_id from the template")
	flag.StringVar(menuSlugFlag, "nav-menu", "", "Deprecated alias for -menu-slug")
	templateFlag := flag.String("template", "", "Path to a template JSON file (required when -year is not given)")
	filmFlag := flag.String("film", "", "Process only the film with this TÍTULO ORIGINAL (case-insensitive)")
	nonInteractiveFlag := flag.Bool("non-interactive", false, "Never prompt; use flags and defaults")
	dryRunFlag := flag.Bool("dry-run", false, "Read the sheet and report what would be processed; no downloads, uploads or writes")
	checkFlag := flag.Bool("check", false, "Check WordPress auth, Turso connectivity and the sheet header row, then exit")
	flag.Parse()

	logger.Init("excentrico-tools-go")
	l := logger.Get()
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing log file: %v\n", err)
		}
	}()
	if logPath := l.GetLogFilePath(); logPath != "" {
		fmt.Fprintf(os.Stderr, "Logging to: %s\n", logPath)
	}
	debug.SetEnabled(*debugFlag)

	if *createConfig {
		op := l.StartOperation("create_config")
		if err := config.CreateDefaultConfig(); err != nil {
			op.Fail("Failed to create configuration", err)
			return fmt.Errorf("create configuration: %w", err)
		}
		op.Complete("Configuration file created successfully")
		return nil
	}

	op := l.StartOperation("load_config")
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		// Not fatal yet: the configuration menu works without a config.
		op.Fail("Configuration error", cfgErr)
	} else {
		op.WithContext("google_sheet_id", cfg.GoogleSheetID)
		op.Complete("Configuration loaded successfully")
	}

	runtime := &RuntimeOptions{
		Menu:           strings.TrimSpace(*menuFlag),
		Year:           strings.TrimSpace(*yearFlag),
		MenuSlug:       strings.TrimSpace(*menuSlugFlag),
		TemplatePath:   strings.TrimSpace(*templateFlag),
		Film:           strings.TrimSpace(*filmFlag),
		NonInteractive: *nonInteractiveFlag,
		DryRun:         *dryRunFlag,
		Check:          *checkFlag,
	}

	if runtime.Menu == "" {
		if runtime.NonInteractive || runtime.Check {
			runtime.Menu = "process"
		} else {
			runtime.Menu = promptMenuSelection()
		}
	}

	switch strings.ToLower(runtime.Menu) {
	case "configuration", "config", "1":
		if runtime.NonInteractive {
			return errors.New("the configuration menu is interactive; use -create-config with -non-interactive")
		}
		return runConfigurationMenu()
	case "process", "process-movies", "2":
		// proceed to processing flow below
	default:
		return fmt.Errorf("unknown -menu %q (valid: configuration, process)", runtime.Menu)
	}

	if cfg == nil {
		return fmt.Errorf("%w\nTo create a default configuration file, run:\n  ./excentrico-tools-go -create-config\nthen edit configuration.json with your settings", cfgErr)
	}

	// Ctrl-C / SIGTERM cancel in-flight HTTP requests and stop before the
	// next film instead of killing the process mid-upload.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if runtime.Check {
		return runCheck(ctx, cfg, l)
	}

	if runtime.Year == "" && !runtime.NonInteractive {
		runtime.Year = promptString("Year filter (enter to skip)")
	}

	// A missing or invalid template is fatal before any network work:
	// processing without one used to panic halfway through a run.
	templateConfig, err := loadTemplateConfig(runtime.TemplatePath, runtime.Year, l)
	if err != nil {
		return err
	}
	metadata := loadMetadata(runtime.Year, l)

	op = l.StartOperation("initialize_application")
	application, err := app.New(cfg)
	if err != nil {
		op.Fail("Failed to initialize application", err)
		return fmt.Errorf("initialize application: %w", err)
	}
	op.Complete("Application initialized successfully")
	defer application.Close()

	// Pick the WordPress navigation menu (from -menu-slug or interactively).
	// The chosen menu's ID overrides menu.menu_id from the template; with no
	// choice the template's menu_id is used as-is.
	menu, err := chooseWordPressMenu(ctx, application, runtime, l)
	if err != nil {
		return err
	}
	if menu != nil {
		runtime.MenuSlug = menu.Slug
		if menu.ID > 0 {
			templateConfig.Menu.MenuId = strconv.Itoa(menu.ID)
		} else {
			l.Progress("WordPress did not report an ID for menu %q; using menu_id %s from the template", menu.Slug, templateConfig.Menu.MenuId)
		}
	}

	mode := ""
	if runtime.DryRun {
		mode = " (dry run)"
	}
	l.Progress("Processing year %q with menu_id %s%s", runtime.Year, templateConfig.Menu.MenuId, mode)

	return application.ProcessFilms(ctx, app.ProcessOptions{
		Year:     runtime.Year,
		Film:     runtime.Film,
		DryRun:   runtime.DryRun,
		Template: templateConfig,
		Metadata: metadata,
	})
}

// runCheck prints one line per connectivity check and fails if any failed.
func runCheck(ctx context.Context, cfg *config.Config, l *logger.Logger) error {
	application, err := app.New(cfg)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	defer application.Close()

	op := l.StartOperation("check").Important()
	failed := 0
	for _, r := range application.Check(ctx) {
		if r.Err != nil {
			failed++
			op.WithContext(r.Name, r.Err.Error())
			l.Progress("FAIL %-13s %v", r.Name, r.Err)
			continue
		}
		op.WithContext(r.Name, r.Detail)
		l.Progress("ok   %-13s %s", r.Name, r.Detail)
	}
	if failed > 0 {
		err := fmt.Errorf("%d check(s) failed", failed)
		op.Fail("Checks failed", err)
		return err
	}
	op.Complete("All checks passed")
	return nil
}

func promptMenuSelection() string {
	fmt.Println("Select a menu:")
	fmt.Println("  1) Configuration")
	fmt.Println("  2) Process movies")
	fmt.Print("Enter choice [1-2] or name: ")
	input := strings.ToLower(readLine(stdin))
	if input == "" {
		return "process"
	}
	return input
}

// promptString prints label and returns the trimmed next line of stdin; an
// empty line, EOF (piped/closed stdin) or a read error all yield "".
func promptString(label string) string {
	fmt.Printf("%s: ", label)
	return readLine(stdin)
}

// readLine returns the next trimmed line from sc, or "" at EOF or on error.
// Unlike fmt.Scanln it keeps spaces inside the line and never leaves the rest
// of a line in the buffer for the next prompt.
func readLine(sc *bufio.Scanner) string {
	if !sc.Scan() {
		if err := sc.Err(); err != nil && err != io.EOF {
			log.Printf("Input error: %v", err)
		}
		return ""
	}
	return strings.TrimSpace(sc.Text())
}

// chooseWordPressMenu resolves the navigation menu to render in the Divi
// template. With -menu-slug it must match an existing WordPress menu (error
// otherwise). Without it the user picks from the list; an empty answer, a
// failure to fetch menus or -non-interactive returns nil so the template's
// menu_id is kept.
func chooseWordPressMenu(ctx context.Context, application *app.App, runtime *RuntimeOptions, l *logger.Logger) (*services.WordPressMenu, error) {
	if runtime.MenuSlug == "" && runtime.NonInteractive {
		return nil, nil
	}
	op := l.StartOperation("list_wordpress_menus")
	op.WithContext("year_filter", runtime.Year)
	menus, err := application.ListWordPressMenus(ctx)
	if err != nil {
		op.Fail("Failed to fetch WordPress menus", err)
		if runtime.MenuSlug != "" {
			return nil, fmt.Errorf("cannot resolve -menu-slug %q: %w", runtime.MenuSlug, err)
		}
		return nil, nil
	}
	op.WithContext("menu_count", len(menus))
	op.Complete(fmt.Sprintf("Fetched %d WordPress menus", len(menus)))

	if runtime.MenuSlug != "" {
		for _, m := range menus {
			if strings.EqualFold(m.Slug, runtime.MenuSlug) {
				return m, nil
			}
		}
		return nil, fmt.Errorf("WordPress menu %q not found (-menu-slug)", runtime.MenuSlug)
	}
	if len(menus) == 0 {
		return nil, nil
	}

	display, matched := filterMenusByYear(menus, runtime.Year)
	switch {
	case runtime.Year == "":
		fmt.Println("Available WordPress menus:")
	case matched:
		fmt.Printf("Available WordPress menus matching year '%s':\n", runtime.Year)
	default:
		fmt.Printf("No menus matched year '%s'. Showing all menus:\n", runtime.Year)
	}
	for idx, m := range display {
		fmt.Printf("  %d) %s (%s) [id %d]\n", idx+1, m.Name, m.Slug, m.ID)
	}
	choice := promptString("Enter choice number or slug (enter to keep the template's menu_id)")
	return findMenu(display, choice), nil
}

// filterMenusByYear returns the menus whose slug or name contains year, and
// whether any matched. With no year, or no match, all menus are returned.
func filterMenusByYear(menus []*services.WordPressMenu, year string) ([]*services.WordPressMenu, bool) {
	year = strings.ToLower(strings.TrimSpace(year))
	if year == "" {
		return menus, false
	}
	var filtered []*services.WordPressMenu
	for _, m := range menus {
		if strings.Contains(strings.ToLower(m.Slug), year) || strings.Contains(strings.ToLower(m.Name), year) {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == 0 {
		return menus, false
	}
	return filtered, true
}

// findMenu resolves a user choice against menus: a 1-based list number, or a
// slug/name (case-insensitive). Returns nil when nothing matches.
func findMenu(menus []*services.WordPressMenu, choice string) *services.WordPressMenu {
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return nil
	}
	if num, err := strconv.Atoi(choice); err == nil {
		if num >= 1 && num <= len(menus) {
			return menus[num-1]
		}
		return nil
	}
	for _, m := range menus {
		if strings.EqualFold(m.Slug, choice) || strings.EqualFold(m.Name, choice) {
			return m
		}
	}
	return nil
}

func runConfigurationMenu() error {
	fmt.Println("Configuration menu")
	// If configuration.json does not exist, offer to create it
	if _, err := os.Stat("configuration.json"); os.IsNotExist(err) {
		fmt.Println("No configuration.json found. Create a default one now? [y/N]")
		if confirmed(promptString("Confirm")) {
			if err := config.CreateDefaultConfig(); err != nil {
				return fmt.Errorf("create configuration: %w", err)
			}
			return nil
		}
		fmt.Println("Skipping creation. Exiting configuration menu.")
		return nil
	}

	fmt.Println("configuration.json exists. Options:")
	fmt.Println("  1) Recreate default configuration.json")
	fmt.Println("  2) Exit")
	if promptString("Enter choice [1-2]") == "1" {
		fmt.Println("This will overwrite configuration.json. Proceed? [y/N]")
		if confirmed(promptString("Confirm")) {
			if err := config.CreateDefaultConfig(); err != nil {
				return fmt.Errorf("recreate configuration: %w", err)
			}
			return nil
		}
		fmt.Println("Cancelled. Exiting configuration menu.")
		return nil
	}
	fmt.Println("Exiting configuration menu.")
	return nil
}

// confirmed reports whether answer is "y" or "yes" (case-insensitive).
func confirmed(answer string) bool {
	a := strings.ToLower(strings.TrimSpace(answer))
	return a == "y" || a == "yes"
}

// loadTemplateConfig loads the Divi template configuration. An explicit path
// (-template) wins; otherwise templates/<year>.json is used. The file is
// decoded strictly and validated, and any problem is returned as an error.
func loadTemplateConfig(explicitPath, year string, l *logger.Logger) (services.TemplateData, error) {
	templatePath := explicitPath
	if templatePath == "" {
		if year == "" {
			return services.TemplateData{}, fmt.Errorf("no template configuration: pass -year <year> (uses templates/<year>.json) or -template <path>")
		}
		templatePath = filepath.Join("templates", year+".json")
	}

	op := l.StartOperation("load_template_config")
	op.WithContext("year", year)
	op.WithContext("template_path", templatePath)

	templateConfig, err := services.LoadTemplateData(templatePath)
	if err != nil {
		op.Fail("Failed to load template configuration", err)
		return services.TemplateData{}, err
	}

	op.Complete(fmt.Sprintf("Loaded template configuration from %s", templatePath))
	return templateConfig, nil
}


func loadMetadata(year string, l *logger.Logger) *models.Metadata {
	if year == "" {
		return nil
	}

	op := l.StartOperation("load_metadata")
	op.WithContext("year", year)
	
	metadataPath := filepath.Join("metadata", year+".json")
	op.WithContext("metadata_path", metadataPath)

	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		op.Warn(&logger.WideEvent{
			Message: fmt.Sprintf("Metadata file not found: %s", metadataPath),
		})
		return nil
	}

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		op.Fail(fmt.Sprintf("Failed to read metadata file %s", metadataPath), err)
		return nil
	}

	var metadataConfig *models.Metadata
	if err := json.Unmarshal(data, &metadataConfig); err != nil {
		op.Fail(fmt.Sprintf("Failed to parse metadata file %s", metadataPath), err)
		return nil
	}

	op.Complete(fmt.Sprintf("Successfully loaded metadata from %s", metadataPath))
	return metadataConfig
}