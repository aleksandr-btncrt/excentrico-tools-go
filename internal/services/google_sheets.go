package services

import (
	"context"
	"fmt"
	"log"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type GoogleSheetsService struct {
	service *sheets.Service
}

func NewGoogleSheetsService(ctx context.Context, credentialsPath string) (*GoogleSheetsService, error) {

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
		service: service,
	}, nil
}

func (s *GoogleSheetsService) ReadRange(spreadsheetID, rangeStr string) ([][]interface{}, error) {
	resp, err := s.service.Spreadsheets.Values.Get(spreadsheetID, rangeStr).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to read range: %v", err)
	}

	return resp.Values, nil
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
