package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type GoogleDriveService struct {
	service *drive.Service
}

func NewGoogleDriveService(ctx context.Context, credentialsPath string) (*GoogleDriveService, error) {

	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %v", err)
	}
	credentials, err := google.CredentialsFromJSON(ctx, data, drive.DriveScope)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %v", err)
	}

	service, err := drive.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		return nil, fmt.Errorf("failed to create drive service: %v", err)
	}

	return &GoogleDriveService{
		service: service,
	}, nil
}

func (s *GoogleDriveService) DownloadFile(fileID, destinationPath string) error {
	resp, err := s.service.Files.Get(fileID).Download()
	if err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	dir := filepath.Dir(destinationPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	dst, err := os.Create(destinationPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	log.Printf("Downloaded file to: %s", destinationPath)
	return nil
}

func (s *GoogleDriveService) ListFiles(folderID string) ([]*drive.File, error) {
	query := fmt.Sprintf("'%s' in parents and trashed=false", folderID)

	files, err := s.service.Files.List().
		Q(query).
		Fields("files(id, name, mimeType, size, createdTime)").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %v", err)
	}

	return files.Files, nil
}
