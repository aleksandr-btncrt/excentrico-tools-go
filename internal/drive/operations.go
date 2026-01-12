package drive

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"excentrico-tools-go/internal/debug"
	"excentrico-tools-go/internal/logger"
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
	l := logger.Get()
	op := l.StartOperation("download_drive_file")
	op.WithDrive("", fileID, filepath.Base(destinationPath))
	op.WithContext("destination_path", destinationPath)

	err := driveService.DownloadFile(fileID, destinationPath)
	if err != nil {
		op.Fail("Failed to download file from Drive", err)
		return fmt.Errorf("download failed: %v", err)
	}

	op.Complete("Successfully downloaded file from Drive")
	return nil
}

// isAllowedFolder checks if a folder name matches one of the allowed folders
// Allowed folders: Background, Featured Image, Stills, Dir
// Uses lowercase comparison for better matching
func isAllowedFolder(folderName string) bool {
	if folderName == "" {
		return false
	}

	allowedFolders := []string{"background", "featured image", "stills", "dir"}
	folderLower := strings.ToLower(strings.TrimSpace(folderName))

	for _, allowed := range allowedFolders {
		if folderLower == allowed {
			return true
		}
	}

	return false
}

// ProcessGoogleDriveFiles processes all files from a Google Drive folder
func ProcessGoogleDriveFiles(filmDir string, driveService *services.GoogleDriveService, imageService *services.ImageService, tursoService *services.TursoService, filmName string, enlacesStr string) error {
	l := logger.Get()
	op := l.StartOperation("process_drive_files")
	
	filmID := utils.SanitizeFilename(filmName)
	op.WithFilm(filmID, filmName, "", "")
	op.WithContext("enlaces_url", enlacesStr)
	op.WithContext("film_dir", filmDir)

	folderID := utils.ExtractFileIDFromURL(enlacesStr)
	if folderID == "" {
		op.Fail("Could not extract folder ID from ENLACES URL", fmt.Errorf("invalid URL format"))
		return fmt.Errorf("could not extract folder ID from ENLACES URL")
	}

	op.WithDrive(folderID, "", "")

	allFiles, err := ListAllFilesRecursively(driveService, folderID)
	if err != nil {
		op.Fail("Failed to list files recursively in folder", err)
		return fmt.Errorf("failed to list files recursively in folder: %v", err)
	}

	var imageFileCount int
	for _, fileInfo := range allFiles {
		if utils.IsImageFile(fileInfo.MimeType) {
			imageFileCount++
		}
	}

	op.WithContext("total_files", len(allFiles))
	op.WithContext("image_files", imageFileCount)
	op.WithContext("other_files", len(allFiles)-imageFileCount)

	// Filter files to only include images from allowed folders
	var filteredFiles []*models.FileWithPath
	skippedCount := 0
	for _, fileInfo := range allFiles {
		if !utils.IsImageFile(fileInfo.MimeType) {
			continue
		}
		// Check if the file is in an allowed folder (using lowercase for matching)
		if isAllowedFolder(fileInfo.FolderName) {
			filteredFiles = append(filteredFiles, fileInfo)
		} else {
			skippedCount++
		}
	}

	op.WithContext("filtered_files", len(filteredFiles))
	op.WithContext("skipped_files", skippedCount)

	var existingFiles []*models.FileWithPath
	var filesToDownload []*models.FileWithPath
	existingCount := 0
	missingCount := 0
	newCount := 0

	err = tursoService.GetDriveFilesMetadata(filmID, &existingFiles)
	if err != nil {
		if strings.Contains(err.Error(), "metadata not found") {
			op.WithContext("existing_metadata", false)
		} else {
			op.Warn(&logger.WideEvent{
				Message: "Failed to load existing Drive metadata",
			})
		}

		for _, fileInfo := range filteredFiles {
			filesToDownload = append(filesToDownload, fileInfo)
		}
		newCount = len(filteredFiles)
	} else {
		op.WithContext("existing_metadata", true)
		op.WithContext("existing_files_count", len(existingFiles))

		existingFileMap := make(map[string]*models.FileWithPath)
		for _, fileInfo := range existingFiles {
			existingFileMap[fileInfo.ID] = fileInfo
		}

		for _, fileInfo := range filteredFiles {
			if _, exists := existingFileMap[fileInfo.ID]; exists {
				var filePath string
				if fileInfo.FolderPath != "" {
					subDir := filepath.Join(filmDir, fileInfo.FolderPath)
					if err := os.MkdirAll(subDir, 0755); err != nil {
						continue
					}
					filePath = filepath.Join(subDir, fileInfo.Name)
				} else {
					filePath = filepath.Join(filmDir, fileInfo.Name)
				}

				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					filesToDownload = append(filesToDownload, fileInfo)
					missingCount++
				} else {
					existingCount++
				}
			} else {
				filesToDownload = append(filesToDownload, fileInfo)
				newCount++
			}
		}
	}

	op.WithContext("files_to_download", len(filesToDownload))
	op.WithContext("existing_on_disk", existingCount)
	op.WithContext("missing_on_disk", missingCount)
	op.WithContext("new_files", newCount)

	downloadedCount := 0
	failedDownloads := 0
	for _, fileInfo := range filesToDownload {
		var filePath string
		if fileInfo.FolderPath != "" {
			subDir := filepath.Join(filmDir, fileInfo.FolderPath)
			if err := os.MkdirAll(subDir, 0755); err != nil {
				failedDownloads++
				continue
			}
			filePath = filepath.Join(subDir, fileInfo.Name)
		} else {
			filePath = filepath.Join(filmDir, fileInfo.Name)
		}

		downloadOp := l.StartOperation("download_drive_file")
		downloadOp.WithFilm(filmID, filmName, "", "")
		downloadOp.WithDrive(folderID, fileInfo.ID, fileInfo.Name)
		downloadOp.WithContext("mime_type", fileInfo.MimeType)
		downloadOp.WithContext("file_path", filePath)
		
		if err := DownloadFile(driveService, fileInfo.ID, filePath); err != nil {
			downloadOp.Fail(fmt.Sprintf("Failed to download %s", fileInfo.Name), err)
			failedDownloads++
			continue
		}
		downloadOp.Complete(fmt.Sprintf("Downloaded: %s", fileInfo.Name))
		downloadedCount++
	}

	op.WithContext("downloaded_count", downloadedCount)
	op.WithContext("failed_downloads", failedDownloads)

	optimizeOp := l.StartOperation("optimize_images")
	optimizeOp.WithFilm(filmID, filmName, "", "")
	processedCount := 0
	failedOptimizations := 0

	for _, fileInfo := range filteredFiles {
		var originalPath string
		if fileInfo.FolderPath != "" {
			originalPath = filepath.Join(filmDir, fileInfo.FolderPath, fileInfo.Name)
		} else {
			originalPath = filepath.Join(filmDir, fileInfo.Name)
		}

		if _, err := os.Stat(originalPath); err == nil {
			optimizedPath := utils.GetOptimizedImagePath(originalPath)

			if _, err := os.Stat(optimizedPath); os.IsNotExist(err) {
				imgOp := l.StartOperation("optimize_single_image")
				imgOp.WithFilm(filmID, filmName, "", "")
				imgOp.WithDrive(folderID, fileInfo.ID, fileInfo.Name)
				
				if err := imageService.ResizeImage(originalPath, optimizedPath); err != nil {
					imgOp.Fail(fmt.Sprintf("Failed to optimize image %s", fileInfo.Name), err)
					failedOptimizations++
					continue
				}

				if originalInfo, err := os.Stat(originalPath); err == nil {
					if optimizedInfo, err := os.Stat(optimizedPath); err == nil {
						originalSize := originalInfo.Size()
						optimizedSize := optimizedInfo.Size()
						
						imgOp.WithImage(originalPath, optimizedPath, originalSize, optimizedSize)
						imgOp.Complete(fmt.Sprintf("Optimized image '%s'", fileInfo.Name))
					}
				}

				processedCount++
			}
		}
	}

	optimizeOp.WithContext("processed_count", processedCount)
	optimizeOp.WithContext("failed_optimizations", failedOptimizations)
	optimizeOp.Complete(fmt.Sprintf("Image optimization completed: %d images processed", processedCount))

	// Only save metadata for filtered files (from allowed folders)
	var imageFiles []*models.FileWithPath
	for _, fileInfo := range filteredFiles {
		imageFiles = append(imageFiles, fileInfo)
	}

	if err := tursoService.SaveDriveFilesMetadata(filmID, imageFiles); err != nil {
		op.Fail("Failed to save Drive media metadata", err)
		return fmt.Errorf("failed to save Drive media metadata: %v", err)
	}

	if len(imageFiles) == 0 {
		op.Warn(&logger.WideEvent{
			Message: "No image files found in folder",
		})
	}

	op.WithCounts(len(imageFiles), len(imageFiles), downloadedCount, skippedCount, 0, processedCount)
	op.Complete(fmt.Sprintf("Successfully processed Drive files for '%s'", filmName))
	return nil
}
