package main

import (
	"flag"
	"log"

	"excentrico-tools-go/internal/app"
	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/debug"
)

func main() {
	createConfig := flag.Bool("create-config", false, "Create a default configuration file")
	year := flag.String("year", "", "Filter by year (e.g., 2024, 2025)")
	debugFlag := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	debug.SetEnabled(*debugFlag)

	if *createConfig {
		if err := config.CreateDefaultConfig(); err != nil {
			log.Fatalf("Failed to create configuration: %v", err)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Printf("Configuration error: %v", err)
		log.Println("")
		log.Println("To create a default configuration file, run:")
		log.Println("  ./excentrico-tools-go -create-config")
		log.Println("")
		log.Println("Then edit the configuration.json file with your settings.")
		return
	}

	log.Printf("Configuration loaded successfully")
	log.Printf("Google Sheet ID: %s", cfg.GoogleSheetID)

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	defer application.Close()

	if err := application.ProcessFilms(*year); err != nil {
		log.Fatalf("Failed to process films: %v", err)
	}

	log.Println("Application completed successfully")
}
