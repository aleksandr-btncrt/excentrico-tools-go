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

func (s *GoogleDriveService) DownloadFile(ctx context.Context, fileID, destinationPath string) error {
	resp, err := s.service.Files.Get(fileID).Context(ctx).Download()
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

// GetFileMetadata fetches metadata (id, name, mimeType) for a given file ID.
func (s *GoogleDriveService) GetFileMetadata(ctx context.Context, fileID string) (*drive.File, error) {
	file, err := s.service.Files.Get(fileID).Fields("id, name, mimeType").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata: %v", err)
	}

	return file, nil
}

// DownloadFileBytes downloads the raw content of a file into memory.
func (s *GoogleDriveService) DownloadFileBytes(ctx context.Context, fileID string) ([]byte, error) {
	resp, err := s.service.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %v", err)
	}

	return data, nil
}

// ListFiles returns every non-trashed file directly inside folderID,
// following nextPageToken (the API returns at most one page per call).
func (s *GoogleDriveService) ListFiles(ctx context.Context, folderID string) ([]*drive.File, error) {
	query := fmt.Sprintf("'%s' in parents and trashed=false", folderID)

	var all []*drive.File
	pageToken := ""
	for {
		call := s.service.Files.List().
			Q(query).
			PageSize(1000).
			Fields("nextPageToken, files(id, name, mimeType, size, createdTime, modifiedTime)")
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		files, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list files: %v", err)
		}
		all = append(all, files.Files...)
		if files.NextPageToken == "" {
			return all, nil
		}
		pageToken = files.NextPageToken
	}
}
