package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	GoogleCredentialsPath string          `json:"google_credentials_path"`
	GoogleSheetID         string          `json:"google_sheet_id"`
	WordPressConfig       WordPressConfig `json:"wordpress_config"`
	ImageConfig           ImageConfig     `json:"image_config"`
	TursoConfig           TursoConfig     `json:"turso_config"`
}

type WordPressConfig struct {
	BaseURL             string `json:"base_url"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	ApplicationPassword string `json:"application_password"`
}

type ImageConfig struct {
	MaxWidth  int `json:"max_width"`
	MaxHeight int `json:"max_height"`
	Quality   int `json:"quality"`
}

type TursoConfig struct {
	DatabaseURL string `json:"database_url"`
	AuthToken   string `json:"auth_token"`
}

func Load() (*Config, error) {
	var configPath string

	if _, err := os.Stat("configuration.json"); err == nil {
		configPath = "configuration.json"
	} else {
		execPath, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("failed to get executable path: %v", err)
		}

		execDir := filepath.Dir(execPath)
		configPath = filepath.Join(execDir, "configuration.json")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			configPath = "configuration.json"
		}
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file %s: %v", configPath, err)
	}

	var cfg Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %v", err)
	}

	if cfg.ImageConfig.MaxWidth == 0 {
		cfg.ImageConfig.MaxWidth = 1920
	}
	if cfg.ImageConfig.MaxHeight == 0 {
		cfg.ImageConfig.MaxHeight = 1080
	}
	if cfg.ImageConfig.Quality == 0 {
		cfg.ImageConfig.Quality = 85
	}
	if cfg.GoogleCredentialsPath == "" {
		cfg.GoogleCredentialsPath = "credentials.json"
	}

	if cfg.WordPressConfig.BaseURL == "" {
		return nil, fmt.Errorf("wordpress base_url is required in configuration")
	}
	if cfg.WordPressConfig.Username == "" {
		return nil, fmt.Errorf("wordpress username is required in configuration")
	}
	if cfg.WordPressConfig.Password == "" && cfg.WordPressConfig.ApplicationPassword == "" {
		return nil, fmt.Errorf("either wordpress password or application_password is required in configuration")
	}

	if cfg.TursoConfig.DatabaseURL == "" {
		return nil, fmt.Errorf("turso database_url is required in configuration")
	}
	if cfg.TursoConfig.AuthToken == "" {
		return nil, fmt.Errorf("turso auth_token is required in configuration")
	}

	credentialsPath := cfg.GoogleCredentialsPath
	if !filepath.IsAbs(credentialsPath) {
		var err error
		credentialsPath, err = filepath.Abs(credentialsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for credentials: %v", err)
		}
	}

	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("google credentials file not found at %s", credentialsPath)
	}

	cfg.GoogleCredentialsPath = credentialsPath

	return &cfg, nil
}

func CreateDefaultConfig() error {
	defaultConfig := Config{
		GoogleCredentialsPath: "credentials.json",
		GoogleSheetID:         "",
		WordPressConfig: WordPressConfig{
			BaseURL:             "https://your-wordpress-site.com",
			Username:            "your-username",
			Password:            "",
			ApplicationPassword: "your-application-password",
		},
		ImageConfig: ImageConfig{
			MaxWidth:  1920,
			MaxHeight: 1080,
			Quality:   85,
		},
		TursoConfig: TursoConfig{
			DatabaseURL: "libsql://your-database-url.turso.io",
			AuthToken:   "your-turso-auth-token",
		},
	}

	configData, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %v", err)
	}

	err = os.WriteFile("configuration.json", configData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write configuration file: %v", err)
	}

	fmt.Println("Created default configuration.json file")
	fmt.Println("Please edit the file with your actual configuration values")

	return nil
}
