package main

import (
	"encoding/json"
	"excentrico-tools-go/internal/app"
	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/debug"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/models"
	"excentrico-tools-go/internal/services"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type RuntimeOptions struct {
	Menu     string
	Year     string
	Template string
	NavMenu  string
}

func main() {
	createConfig := flag.Bool("create-config", false, "Create a default configuration file")
	yearFlag := flag.String("year", "", "Filter by year (e.g., 2024, 2025)")
	debugFlag := flag.Bool("debug", false, "Enable debug logging")
	menuFlag := flag.String("menu", "", "Action to run: configuration | process")
	navMenuFlag := flag.String("nav-menu", "", "Navigation menu to use (from WordPress)")
	flag.Parse()

	// Initialize logger
	logger.Init("excentrico-tools-go")
	l := logger.Get()
	if *debugFlag {
		l.SetSampleRate(1.0) // Log everything in debug mode
	}
	
	// Log file path information
	if logPath := l.GetLogFilePath(); logPath != "" {
		fmt.Fprintf(os.Stderr, "Logging to: %s\n", logPath)
	}
	
	// Ensure log file is closed on exit
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing log file: %v\n", err)
		}
	}()

	debug.SetEnabled(*debugFlag)

	if *createConfig {
		op := l.StartOperation("create_config")
		if err := config.CreateDefaultConfig(); err != nil {
			op.Fail("Failed to create configuration", err)
			log.Fatalf("Failed to create configuration: %v", err)
		}
		op.Complete("Configuration file created successfully")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		op := l.StartOperation("load_config")
		op.WithError(err)
		op.Fail("Configuration error", err)
		fmt.Println("")
		fmt.Println("To create a default configuration file, run:")
		fmt.Println("  ./excentrico-tools-go -create-config")
		fmt.Println("")
		fmt.Println("Then edit the configuration.json file with your settings.")
		// Even if configuration is missing, allow entering the configuration menu
	}

	if cfg != nil {
		op := l.StartOperation("load_config")
		op.WithContext("google_sheet_id", cfg.GoogleSheetID)
		op.Complete("Configuration loaded successfully")
	}

	// Collect runtime options (from flags or interactive prompts)
	runtime := &RuntimeOptions{
		Menu:    strings.TrimSpace(*menuFlag),
		Year:    strings.TrimSpace(*yearFlag),
		NavMenu: strings.TrimSpace(*navMenuFlag),
	}

	// Back-compat: if -nav-menu was provided, use it as the template (menu slug)
	if runtime.Template == "" && runtime.NavMenu != "" {
		runtime.Template = runtime.NavMenu
	}

	if runtime.Menu == "" {
		runtime.Menu = promptMenuSelection()
	}

	switch strings.ToLower(runtime.Menu) {
	case "configuration", "config", "1":
		runConfigurationMenu()
		return
	case "process", "process-movies", "2":
		// proceed to processing flow below
	default:
		op := l.StartOperation("menu_selection")
		op.WithContext("menu_option", runtime.Menu)
		op.Fail(fmt.Sprintf("Unknown menu option '%s'", runtime.Menu), fmt.Errorf("valid options: configuration, process"))
		return
	}

	if cfg == nil {
		op := l.StartOperation("process_films")
		op.Fail("Configuration required", fmt.Errorf("configuration is required to process movies"))
		return
	}

	if runtime.Year == "" {
		runtime.Year = promptString("Year filter (enter to skip)")
	}

	// Search for and load year-based template configuration
	var templateConfig *services.TemplateData
	if runtime.Year != "" {
		templateConfig = loadYearTemplateConfig(runtime.Year, l)
		if templateConfig != nil {
			op := l.StartOperation("load_template_config")
			op.WithContext("year", runtime.Year)
			op.WithContext("template_path", fmt.Sprintf("templates/%s.json", runtime.Year))
			op.Complete(fmt.Sprintf("Loaded template configuration for year %s", runtime.Year))
		} else {
			op := l.StartOperation("load_template_config")
			op.WithContext("year", runtime.Year)
			op.Warn(&logger.WideEvent{
				Message: fmt.Sprintf("No template configuration found for year %s", runtime.Year),
			})
		}
	}

	var metadata = loadMetadata(runtime.Year, l)

	if runtime.Template == "" {
		// Fetch WordPress menus and select one as the template (menu slug)
		op := l.StartOperation("list_wordpress_menus")
		applicationTmp, err := app.New(cfg)
		if err != nil {
			op.Fail("Failed to initialize application for menu listing", err)
		} else {
			defer applicationTmp.Close()
			var menus []*services.WordPressMenu
			menus, err = applicationTmp.ListWordPressMenus()
			if err != nil {
				op.Fail("Failed to fetch WordPress menus", err)
			} else {
				op.WithContext("menu_count", len(menus))
				op.WithContext("year_filter", runtime.Year)
				op.Complete(fmt.Sprintf("Fetched %d WordPress menus", len(menus)))
			}
			if len(menus) > 0 {
				display := menus
				if strings.TrimSpace(runtime.Year) != "" {
					yearLower := strings.ToLower(runtime.Year)
					var filtered []*services.WordPressMenu
					for _, m := range menus {
						if strings.Contains(strings.ToLower(m.Slug), yearLower) || strings.Contains(strings.ToLower(m.Name), yearLower) {
							filtered = append(filtered, m)
						}
					}
					if len(filtered) > 0 {
						display = filtered
						fmt.Printf("Available WordPress menus matching year '%s':\n", runtime.Year)
					} else {
						fmt.Printf("No menus matched year '%s'. Showing all menus:\n", runtime.Year)
					}
				} else {
					fmt.Println("Available WordPress menus:")
				}

				for idx, m := range display {
					fmt.Printf("  %d) %s (%s)\n", idx+1, m.Name, m.Slug)
				}
				choice := promptString("Enter choice number or slug (enter to type slug manually)")
				choiceLower := strings.ToLower(strings.TrimSpace(choice))
				if choiceLower != "" {
					var num int
					if _, err := fmt.Sscanf(choiceLower, "%d", &num); err == nil {
						if num >= 1 && num <= len(display) {
							runtime.Template = display[num-1].Slug
						}
					} else {
						for _, m := range display {
							if strings.EqualFold(m.Slug, choiceLower) || strings.EqualFold(m.Name, choice) {
								runtime.Template = m.Slug
								break
							}
						}
					}
				}
			}
		}
	}

	// No separate nav menu prompt; template now represents the selected WP menu

	op := l.StartOperation("initialize_application")
	application, err := app.New(cfg)
	if err != nil {
		op.Fail("Failed to initialize application", err)
		log.Fatalf("Failed to initialize application: %v", err)
	}
	op.Complete("Application initialized successfully")
	defer application.Close()

	if strings.TrimSpace(runtime.Template) == "" {
		op := l.StartOperation("process_films")
		op.Fail("No WordPress menu selected", fmt.Errorf("aborting"))
		return
	}
	
	op = l.StartOperation("process_films")
	op.WithContext("template", runtime.Template)
	op.WithContext("year", runtime.Year)
	op.Complete(fmt.Sprintf("Starting film processing with template '%s'", runtime.Template))
	// Template is the WP menu slug

	if err := application.ProcessFilms(runtime.Year, templateConfig, metadata); err != nil {
		op := l.StartOperation("process_films")
		op.WithContext("template", runtime.Template)
		op.WithContext("year", runtime.Year)
		op.Fail("Failed to process films", err)
		log.Fatalf("Failed to process films: %v", err)
	}

	op = l.StartOperation("process_films")
	op.WithContext("template", runtime.Template)
	op.WithContext("year", runtime.Year)
	op.Complete("Application completed successfully")
}

func promptMenuSelection() string {
	fmt.Println("Select a menu:")
	fmt.Println("  1) Configuration")
	fmt.Println("  2) Process movies")
	fmt.Print("Enter choice [1-2] or name: ")
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		// handle empty input (e.g., just Enter)
		if err.Error() == "unexpected newline" {
			return "process"
		}
		log.Printf("Input error: %v", err)
		return "process"
	}
	return strings.ToLower(strings.TrimSpace(input))
}

func promptString(label string) string {
	fmt.Printf("%s: ", label)
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		if err.Error() == "unexpected newline" {
			return ""
		}
		log.Printf("Input error: %v", err)
		return ""
	}
	return strings.TrimSpace(input)
}

func runConfigurationMenu() {
	fmt.Println("Configuration menu")
	// If configuration.json does not exist, offer to create it
	if _, err := os.Stat("configuration.json"); os.IsNotExist(err) {
		fmt.Println("No configuration.json found. Create a default one now? [y/N]")
		answer := promptString("Confirm")
		if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
			if err := config.CreateDefaultConfig(); err != nil {
				log.Fatalf("Failed to create configuration: %v", err)
			}
			return
		}
		fmt.Println("Skipping creation. Exiting configuration menu.")
		return
	}

	fmt.Println("configuration.json exists. Options:")
	fmt.Println("  1) Recreate default configuration.json")
	fmt.Println("  2) Exit")
	choice := promptString("Enter choice [1-2]")
	if choice == "1" {
		fmt.Println("This will overwrite configuration.json. Proceed? [y/N]")
		answer := promptString("Confirm")
		if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
			if err := config.CreateDefaultConfig(); err != nil {
				log.Fatalf("Failed to recreate configuration: %v", err)
			}
			return
		}
		fmt.Println("Cancelled. Exiting configuration menu.")
		return
	}
	fmt.Println("Exiting configuration menu.")
}

// loadYearTemplateConfig searches for and loads a JSON template configuration file
// based on the provided year from the templates folder
func loadYearTemplateConfig(year string, l *logger.Logger) *services.TemplateData {
	if year == "" {
		return nil
	}

	op := l.StartOperation("load_template_config")
	op.WithContext("year", year)
	
	// Construct the template file path
	templatePath := filepath.Join("templates", year+".json")
	op.WithContext("template_path", templatePath)

	// Check if the file exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		op.Warn(&logger.WideEvent{
			Message: fmt.Sprintf("Template file not found: %s", templatePath),
		})
		return nil
	}

	// Read the template file
	data, err := os.ReadFile(templatePath)
	if err != nil {
		op.Fail(fmt.Sprintf("Failed to read template file %s", templatePath), err)
		return nil
	}

	// Parse the JSON
	var templateConfig *services.TemplateData
	if err := json.Unmarshal(data, &templateConfig); err != nil {
		op.Fail(fmt.Sprintf("Failed to parse template file %s", templatePath), err)
		return nil
	}

	op.Complete(fmt.Sprintf("Successfully loaded template configuration from %s", templatePath))
	return templateConfig
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