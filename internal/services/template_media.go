package services

import (
	"path/filepath"
	"strings"

	"excentrico-tools-go/internal/debug"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/models"
)

// Media selection for the Divi template: gallery stills, background image
// and director portraits. Every lookup goes through the per-film MediaCache.

// filterStillsImages filters imageIds to only include images from the Stills folder
// Uses drive metadata to determine which images are from the Stills folder
func (s *DiviTemplateService) filterStillsImages(imageIds []int, tursoService MetadataStore, filmID string) []int {
	if tursoService == nil || len(imageIds) == 0 || filmID == "" {
		return []int{}
	}

	// Get WordPress media metadata to map image IDs to file paths
	var wpMediaMetadata []models.MediaInfo
	err := tursoService.GetMetadata(filmID, MetadataWordPressMedia, &wpMediaMetadata)
	if err != nil {
		// If no WordPress media metadata, return empty
		return []int{}
	}

	// Create a map from WordPress media ID to file path
	mediaIDToFilePath := make(map[int]string)
	for _, media := range wpMediaMetadata {
		if media.ID != 0 && media.FilePath != "" {
			mediaIDToFilePath[media.ID] = media.FilePath
		}
	}

	// Get drive files metadata
	var driveFiles []*models.FileWithPath
	err = GetDriveFilesMetadata(tursoService, filmID, &driveFiles)
	if err != nil {
		// If no drive metadata, return empty
		return []int{}
	}

	// Create a map from filename (without _web.jpg) to FolderName
	filenameToFolder := make(map[string]string)
	for _, file := range driveFiles {
		// Remove _web.jpg suffix if present, and normalize
		filename := strings.TrimSuffix(strings.ToLower(file.Name), "_web.jpg")
		// Also try without any extension
		if dot := strings.LastIndex(filename, "."); dot != -1 {
			filenameWithoutExt := filename[:dot]
			filenameToFolder[filenameWithoutExt] = strings.ToLower(file.FolderName)
		}
		filenameToFolder[filename] = strings.ToLower(file.FolderName)
	}

	// Filter imageIds to only include those from Stills folder
	var stillsIds []int
	for _, id := range imageIds {
		filePath, exists := mediaIDToFilePath[id]
		if !exists {
			continue
		}

		// Extract filename from file path
		fileName := filepath.Base(filePath)
		// Remove _web.jpg suffix
		fileName = strings.TrimSuffix(strings.ToLower(fileName), "_web.jpg")
		// Remove extension
		if dot := strings.LastIndex(fileName, "."); dot != -1 {
			fileName = fileName[:dot]
		}

		// Check if this file is from Stills folder
		if folderName, exists := filenameToFolder[fileName]; exists {
			if folderName == "stills" {
				stillsIds = append(stillsIds, id)
			}
		}
	}

	return stillsIds
}

// selectBackgroundImageURL attempts to pick a background image URL from media
// Preference order by media title/filename/alt text contains: background, header, fondo, bg
// Falls back to the first media URL if any
func (s *DiviTemplateService) selectBackgroundImageURL(imageIds []int, cache *MediaCache) string {
	if len(imageIds) == 0 {
		return ""
	}

	keywords := []string{"background", "header", "fondo", "bg"}

	// First pass: try to match keywords
	for _, id := range imageIds {
		media, ok := cache.Get(id)
		if !ok {
			continue
		}
		if MediaMatchesAny(media, keywords) {
			return media.SourceURL
		}
	}

	// Second pass: fallback to first media URL
	if first, ok := cache.Get(imageIds[0]); ok {
		return first.SourceURL
	}
	return ""
}

// MediaMatchesAny reports whether the media title, file name or alt text
// contains one of keywords (case-insensitive; keywords are lower case).
func MediaMatchesAny(media *WordPressMedia, keywords []string) bool {
	title := strings.ToLower(media.Title.String())
	alt := strings.ToLower(media.AltText)
	filename := MediaFilenameStem(media.SourceURL)
	for _, kw := range keywords {
		if strings.Contains(title, kw) || strings.Contains(filename, kw) || strings.Contains(alt, kw) {
			return true
		}
	}
	return false
}

// findDirectorImage returns the URL of the image whose title, file name or
// alt text contains the director's name; failing that, one whose title or
// file name contains a part (longer than 2 characters) of the name.
func (s *DiviTemplateService) findDirectorImage(directorName string, imageIds []int, cache *MediaCache) string {
	if len(imageIds) == 0 {
		return ""
	}

	cleanName := strings.ToLower(strings.TrimSpace(directorName))

	for _, imageID := range imageIds {
		media, ok := cache.Get(imageID)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(media.Title.String()), cleanName) ||
			strings.Contains(MediaFilenameStem(media.SourceURL), cleanName) ||
			strings.Contains(strings.ToLower(media.AltText), cleanName) {
			debug.Printf("Found director image for '%s': %s", directorName, media.SourceURL)
			return media.SourceURL
		}
	}

	nameParts := strings.Fields(cleanName)
	if len(nameParts) > 1 {
		for _, imageID := range imageIds {
			media, ok := cache.Get(imageID)
			if !ok {
				continue
			}
			title := strings.ToLower(media.Title.String())
			filename := MediaFilenameStem(media.SourceURL)
			for _, part := range nameParts {
				if len(part) > 2 && (strings.Contains(title, part) || strings.Contains(filename, part)) {
					debug.Printf("Found director image for '%s' (partial match with '%s'): %s", directorName, part, media.SourceURL)
					return media.SourceURL
				}
			}
		}
	}

	logger.Get().Warnf("find_director_image", "No director image found for '%s'", directorName)
	return ""
}
