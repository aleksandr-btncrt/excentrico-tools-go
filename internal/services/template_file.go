package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"excentrico-tools-go/internal/logger"
)

// divi_template.json writer (Divi portability format).

// DiviImageData is one entry of the "images" map in divi_template.json.
type DiviImageData struct {
	Encoded string `json:"encoded"`
	URL     string `json:"url"`
	ID      int    `json:"id"`
}

// DiviTemplateFile is the divi_template.json document imported through
// Divi Portability.
type DiviTemplateFile struct {
	Context      string                   `json:"context"`
	Data         map[string]string        `json:"data"`
	Presets      map[string]any           `json:"presets"`
	GlobalColors [][]any                  `json:"global_colors"`
	Images       map[string]DiviImageData `json:"images"`
	Thumbnails   []any                    `json:"thumbnails"`
}

// SaveDiviTemplateToFile writes <filmDir>/divi_template.json (Divi
// portability format) for already generated shortcodes. Its "images" list
// holds every cached media item; with EmbedImages each image is downloaded
// through downloader and embedded as base64.
func (s *DiviTemplateService) SaveDiviTemplateToFile(ctx context.Context, shortcodes string, cache *MediaCache, downloader MediaClient, filmDir string, wordpressPostID int) error {
	// Use WordPress Post ID instead of film title for better consistency
	projectID := fmt.Sprintf("%d", wordpressPostID)

	images := s.imagesData(ctx, cache, downloader)

	templateFile := &DiviTemplateFile{
		Context: "et_builder",
		Data: map[string]string{
			projectID: shortcodes,
		},
		Presets: map[string]any{
			"et_pb_row": map[string]any{
				"presets": map[string]any{
					"_initial": map[string]any{
						"name":    "Fila Preset 1",
						"version": BuilderVersion,
						"settings": map[string]any{
							"use_custom_gutter": "off",
							"gutter_width":      "1",
							"width":             "90%",
							"module_alignment":  "center",
						},
					},
				},
				"default": "_initial",
			},
		},
		GlobalColors: [][]any{
			{"gcid-primary-color", map[string]any{"color": ColorDark, "active": "yes"}},
			{"gcid-secondary-color", map[string]any{"color": ColorPrimary, "active": "yes"}},
			{"gcid-heading-color", map[string]any{"color": ColorSecondary, "active": "yes"}},
			{"gcid-body-color", map[string]any{"color": ColorBody, "active": "yes"}},
		},
		Images:     images,
		Thumbnails: []any{},
	}

	templatePath := filepath.Join(filmDir, "divi_template.json")
	templateJSON, err := json.MarshalIndent(templateFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template file: %v", err)
	}

	if err := os.WriteFile(templatePath, templateJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write template file: %v", err)
	}

	logger.Get().Infof("save_divi_template", "Saved Divi template to: %s", templatePath)

	return nil
}

// imagesData builds the "images" entry of the template file, keyed by URL.
func (s *DiviTemplateService) imagesData(ctx context.Context, cache *MediaCache, downloader MediaClient) map[string]DiviImageData {
	images := make(map[string]DiviImageData)
	for _, id := range cache.IDs() {
		media, _ := cache.Get(id)
		encoded := ""
		if s.EmbedImages && downloader != nil {
			data, err := downloader.Download(ctx, media.SourceURL)
			if err != nil {
				logger.Get().Warnf("fetch_wordpress_images", "Failed to encode image %d: %v", id, err)
			} else {
				encoded = base64.StdEncoding.EncodeToString(data)
			}
		}
		images[media.SourceURL] = DiviImageData{
			Encoded: encoded,
			URL:     media.SourceURL,
			ID:      id,
		}
	}
	return images
}
