package drive

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"excentrico-tools-go/internal/debug"
	"excentrico-tools-go/internal/models"
	"excentrico-tools-go/internal/services"
	"excentrico-tools-go/internal/utils"
)

// ListAllFilesRecursively lists all files in a folder and its subfolders
func ListAllFilesRecursively(driveService *services.GoogleDriveService, folderID string) ([]*models.FileWithPath, error) {
	return ListAllFilesRecursivelyWithPath(driveService, folderID, "")
}

// ListAllFilesRecursivelyWithPath lists all files recursively with the current path context
func ListAllFilesRecursivelyWithPath(driveService *services.GoogleDriveService, folderID string, currentPath string) ([]*models.FileWithPath, error) {
	var allFiles []*models.FileWithPath

	debug.Printf("Listing contents of folder ID: %s (path: %s)", folderID, currentPath)

	items, err := driveService.ListFiles(folderID)
	if err != nil {
		debug.Printf("Error listing folder %s: %v", folderID, err)
		return nil, err
	}

	debug.Printf("Found %d items in folder %s", len(items), folderID)

	for i, item := range items {
		debug.Printf("Item %d: %s (type: %s)", i+1, item.Name, item.MimeType)

		if item.MimeType == "application/vnd.google-apps.folder" {
			debug.Printf("Processing subfolder: %s", item.Name)
			var newPath string
			if currentPath == "" {
				newPath = item.Name
			} else {
				newPath = currentPath + "/" + item.Name
			}

			subFiles, err := ListAllFilesRecursivelyWithPath(driveService, item.Id, newPath)
			if err != nil {
				log.Printf("Warning: Failed to list files in subfolder '%s': %v", item.Name, err)
				continue
			}
			debug.Printf("Found %d files in subfolder '%s'", len(subFiles), item.Name)
			allFiles = append(allFiles, subFiles...)
		} else {
			debug.Printf("Adding file: %s (path: %s)", item.Name, currentPath)

			folderName := ""
			if currentPath != "" {
				pathParts := strings.Split(currentPath, "/")
				folderName = pathParts[len(pathParts)-1]
			}

			fileWithPath := &models.FileWithPath{
				ID:           item.Id,
				Name:         item.Name,
				MimeType:     item.MimeType,
				Size:         fmt.Sprintf("%d", item.Size),
				CreatedTime:  item.CreatedTime,
				ModifiedTime: item.ModifiedTime,
				FolderPath:   currentPath,
				FolderName:   folderName,
			}
			allFiles = append(allFiles, fileWithPath)
		}
	}

	debug.Printf("Returning %d total files from folder %s", len(allFiles), folderID)
	return allFiles, nil
}

// DownloadFile downloads a file from Google Drive to the specified destination
func DownloadFile(driveService *services.GoogleDriveService, fileID, destinationPath string) error {
	log.Printf("Downloading file ID %s to %s", fileID, destinationPath)

	err := driveService.DownloadFile(fileID, destinationPath)
	if err != nil {
		return fmt.Errorf("download failed: %v", err)
	}

	return nil
}

// ProcessGoogleDriveFiles processes all files from a Google Drive folder
func ProcessGoogleDriveFiles(filmDir string, driveService *services.GoogleDriveService, imageService *services.ImageService, tursoService *services.TursoService, filmName string, enlacesStr string) error {
	log.Printf("Processing ENLACES for film '%s': %s", filmName, enlacesStr)

	folderID := utils.ExtractFileIDFromURL(enlacesStr)
	if folderID == "" {
		return fmt.Errorf("could not extract folder ID from ENLACES URL")
	}

	log.Printf("Extracted folder ID: %s", folderID)

	allFiles, err := ListAllFilesRecursively(driveService, folderID)
	if err != nil {
		return fmt.Errorf("failed to list files recursively in folder: %v", err)
	}

	var imageFileCount int
	for _, fileInfo := range allFiles {
		if utils.IsImageFile(fileInfo.MimeType) {
			imageFileCount++
		}
	}

	log.Printf("Found %d total files (%d images, %d other files) in folder for '%s'", len(allFiles), imageFileCount, len(allFiles)-imageFileCount, filmName)

	filmID := utils.SanitizeFilename(filmName)

	var existingFiles []*models.FileWithPath
	var filesToDownload []*models.FileWithPath

	err = tursoService.GetDriveFilesMetadata(filmID, &existingFiles)
	if err != nil {
		if strings.Contains(err.Error(), "metadata not found") {
			log.Printf("No existing Drive metadata found for '%s', will download all image files", filmName)
		} else {
			log.Printf("Failed to load existing Drive metadata: %v", err)
		}

		for _, fileInfo := range allFiles {
			if utils.IsImageFile(fileInfo.MimeType) {
				filesToDownload = append(filesToDownload, fileInfo)
			}
		}
	} else {
		log.Printf("Found existing metadata with %d files", len(existingFiles))

		existingFileMap := make(map[string]*models.FileWithPath)
		for _, fileInfo := range existingFiles {
			existingFileMap[fileInfo.ID] = fileInfo
		}

		for _, fileInfo := range allFiles {
			if !utils.IsImageFile(fileInfo.MimeType) {
				continue
			}

			if _, exists := existingFileMap[fileInfo.ID]; exists {
				var filePath string
				if fileInfo.FolderPath != "" {
					subDir := filepath.Join(filmDir, fileInfo.FolderPath)
					if err := os.MkdirAll(subDir, 0755); err != nil {
						log.Printf("Failed to create subdirectory %s: %v", subDir, err)
						continue
					}
					filePath = filepath.Join(subDir, fileInfo.Name)
				} else {
					filePath = filepath.Join(filmDir, fileInfo.Name)
				}

				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					log.Printf("Image exists in metadata but missing on disk, will re-download: %s", fileInfo.Name)
					filesToDownload = append(filesToDownload, fileInfo)
				} else {
					log.Printf("Image already exists and up to date: %s", fileInfo.Name)
				}
			} else {
				log.Printf("New image found, will download: %s", fileInfo.Name)
				filesToDownload = append(filesToDownload, fileInfo)
			}
		}
	}

	log.Printf("Will download %d image files for '%s'", len(filesToDownload), filmName)

	for _, fileInfo := range filesToDownload {
		var filePath string
		if fileInfo.FolderPath != "" {
			subDir := filepath.Join(filmDir, fileInfo.FolderPath)
			if err := os.MkdirAll(subDir, 0755); err != nil {
				log.Printf("Failed to create subdirectory %s: %v", subDir, err)
				continue
			}
			filePath = filepath.Join(subDir, fileInfo.Name)
		} else {
			filePath = filepath.Join(filmDir, fileInfo.Name)
		}

		log.Printf("Downloading image: %s (MIME: %s)", fileInfo.Name, fileInfo.MimeType)
		if err := DownloadFile(driveService, fileInfo.ID, filePath); err != nil {
			log.Printf("Failed to download %s: %v", fileInfo.Name, err)
			continue
		}
		log.Printf("Downloaded: %s", filePath)
	}

	log.Printf("Starting image optimization for '%s'", filmName)
	processedCount := 0

	for _, fileInfo := range allFiles {
		if utils.IsImageFile(fileInfo.MimeType) {
			var originalPath string
			if fileInfo.FolderPath != "" {
				originalPath = filepath.Join(filmDir, fileInfo.FolderPath, fileInfo.Name)
			} else {
				originalPath = filepath.Join(filmDir, fileInfo.Name)
			}

			if _, err := os.Stat(originalPath); err == nil {
				optimizedPath := utils.GetOptimizedImagePath(originalPath)

				if _, err := os.Stat(optimizedPath); os.IsNotExist(err) {
					log.Printf("Optimizing image: %s", fileInfo.Name)
					if err := imageService.ResizeImage(originalPath, optimizedPath); err != nil {
						log.Printf("Failed to optimize image %s: %v", fileInfo.Name, err)
						continue
					}

					if originalInfo, err := os.Stat(originalPath); err == nil {
						if optimizedInfo, err := os.Stat(optimizedPath); err == nil {
							originalSize := originalInfo.Size()
							optimizedSize := optimizedInfo.Size()
							reduction := float64(originalSize-optimizedSize) / float64(originalSize) * 100

							log.Printf("Optimized image '%s' -> JPG: %d bytes -> %d bytes (%.1f%% reduction)",
								fileInfo.Name, originalSize, optimizedSize, reduction)
						}
					}

					processedCount++
				}
			}
		}
	}

	log.Printf("Image optimization completed for '%s': %d images processed", filmName, processedCount)

	var imageFiles []*models.FileWithPath
	for _, fileInfo := range allFiles {
		if utils.IsImageFile(fileInfo.MimeType) {
			imageFiles = append(imageFiles, fileInfo)
		}
	}

	if err := tursoService.SaveDriveFilesMetadata(filmID, imageFiles); err != nil {
		return fmt.Errorf("failed to save Drive media metadata: %v", err)
	}

	if len(imageFiles) == 0 {
		log.Printf("Warning: No image files found in folder for '%s'", filmName)
	}

	log.Printf("Updated Drive media metadata for '%s' (%d image files) saved to database", filmName, len(imageFiles))
	return nil
}
