package services

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/xuri/excelize/v2"
)

// xlsxMimeType is the mimeType Drive assigns to native Excel (.xlsx) files.
// The Sheets API cannot read values from files with this mimeType because
// they are not native Google Sheets documents (see issue #1).
const xlsxMimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

type GoogleSheetsService struct {
	service      *sheets.Service
	driveService *GoogleDriveService
}

// NewGoogleSheetsService creates a Google Sheets client. driveService is used
// as a fallback to detect and read files that are actually .xlsx documents
// stored in Drive rather than native Google Sheets. It may be nil, in which
// case ReadRange behaves exactly as before (Sheets API only).
func NewGoogleSheetsService(ctx context.Context, credentialsPath string, driveService *GoogleDriveService) (*GoogleSheetsService, error) {

	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %v", err)
	}
	credentials, err := google.CredentialsFromJSON(ctx, data, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %v", err)
	}

	service, err := sheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		return nil, fmt.Errorf("failed to create sheets service: %v", err)
	}

	return &GoogleSheetsService{
		service:      service,
		driveService: driveService,
	}, nil
}

// ReadRange reads the values from spreadsheetID/rangeStr. If the file is a
// native Google Sheets document, it uses the Sheets API directly. If the
// file is actually an .xlsx document stored in Drive (which the Sheets API
// cannot read - see issue #1), it downloads the file via Drive and parses
// it locally instead.
func (s *GoogleSheetsService) ReadRange(spreadsheetID, rangeStr string) ([][]interface{}, error) {
	if s.driveService != nil {
		if file, err := s.driveService.GetFileMetadata(spreadsheetID); err == nil && file.MimeType == xlsxMimeType {
			return s.readRangeFromXLSX(spreadsheetID, rangeStr)
		}
	}

	resp, err := s.service.Spreadsheets.Values.Get(spreadsheetID, rangeStr).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to read range: %v", err)
	}

	return resp.Values, nil
}

// readRangeFromXLSX downloads the file identified by fileID from Drive and
// parses it locally as an xlsx document.
func (s *GoogleSheetsService) readRangeFromXLSX(fileID, rangeStr string) ([][]interface{}, error) {
	data, err := s.driveService.DownloadFileBytes(fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to download xlsx file: %v", err)
	}

	return parseXLSXRange(data, rangeStr)
}

// parseXLSXRange reads the sheet named in rangeStr (before the "!"
// separator) from raw xlsx bytes using excelize, returning the rows in the
// same shape ReadRange normally returns via the Sheets API.
func parseXLSXRange(data []byte, rangeStr string) ([][]interface{}, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to open xlsx file: %v", err)
	}
	defer f.Close()

	sheetName := rangeStr
	if idx := strings.Index(rangeStr, "!"); idx != -1 {
		sheetName = rangeStr[:idx]
	}
	sheetName = strings.Trim(sheetName, "'")

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet %q from xlsx file: %v", sheetName, err)
	}

	values := make([][]interface{}, len(rows))
	for i, row := range rows {
		converted := make([]interface{}, len(row))
		for j, cell := range row {
			converted[j] = cell
		}
		values[i] = converted
	}

	return values, nil
}

func (s *GoogleSheetsService) WriteRange(spreadsheetID, rangeStr string, values [][]interface{}) error {
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	_, err := s.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("RAW").
		Do()
	if err != nil {
		return fmt.Errorf("failed to write range: %v", err)
	}

	return nil
}

func (s *GoogleSheetsService) AppendRow(spreadsheetID, rangeStr string, values []interface{}) error {
	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{values},
	}

	_, err := s.service.Spreadsheets.Values.Append(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Do()
	if err != nil {
		return fmt.Errorf("failed to append row: %v", err)
	}

	return nil
}

func (s *GoogleSheetsService) CreateSpreadsheet(title string) (*sheets.Spreadsheet, error) {
	spreadsheet := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: title,
		},
	}

	resp, err := s.service.Spreadsheets.Create(spreadsheet).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create spreadsheet: %v", err)
	}

	log.Printf("Created spreadsheet: %s (ID: %s)", title, resp.SpreadsheetId)
	return resp, nil
}

func (s *GoogleSheetsService) GetSpreadsheet(spreadsheetID string) (*sheets.Spreadsheet, error) {
	resp, err := s.service.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get spreadsheet: %v", err)
	}

	return resp, nil
}
