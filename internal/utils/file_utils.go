package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// SanitizeFilename cleans a filename to be safe for filesystem use
func SanitizeFilename(name string) string {
	// Replace invalid characters with underscores
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	sanitized := reg.ReplaceAllString(name, "_")

	// Trim whitespace and dots
	sanitized = strings.TrimSpace(sanitized)
	sanitized = strings.Trim(sanitized, ".")

	// Replace multiple whitespace with single space
	reg = regexp.MustCompile(`\s+`)
	sanitized = reg.ReplaceAllString(sanitized, " ")

	// Provide default name if empty
	if sanitized == "" {
		sanitized = "unnamed_film"
	}

	return sanitized
}

// GetOptimizedImagePath returns the path for the optimized version of an image
func GetOptimizedImagePath(originalPath string) string {
	dir := filepath.Dir(originalPath)
	filename := filepath.Base(originalPath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	optimizedFilename := nameWithoutExt + "_web.jpg"
	return filepath.Join(dir, optimizedFilename)
}

// IsImageFile checks if a MIME type represents an image file
func IsImageFile(mimeType string) bool {
	imageTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/webp",
		"image/bmp",
		"image/tiff",
	}

	for _, imgType := range imageTypes {
		if strings.EqualFold(mimeType, imgType) {
			return true
		}
	}
	return false
}

// ExtractFileIDFromURL extracts Google Drive file ID from various URL formats
func ExtractFileIDFromURL(url string) string {
	patterns := []string{
		`/drive/u/\d+/folders/([a-zA-Z0-9-_]+)`,
		`/drive/folders/([a-zA-Z0-9-_]+)`,
		`/file/d/([a-zA-Z0-9-_]+)`,
		`id=([a-zA-Z0-9-_]+)`,
		`/d/([a-zA-Z0-9-_]+)`,
		`/folders/([a-zA-Z0-9-_]+)`,
	}

	for _, pattern := range patterns {
		reg := regexp.MustCompile(pattern)
		matches := reg.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}
