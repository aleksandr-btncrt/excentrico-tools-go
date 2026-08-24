package services

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func buildTestXLSX(t *testing.T, sheetName string, rows [][]string) []byte {
	t.Helper()

	f := excelize.NewFile()
	defer f.Close()

	if sheetName != "Sheet1" {
		if _, err := f.NewSheet(sheetName); err != nil {
			t.Fatalf("failed to create sheet: %v", err)
		}
		f.DeleteSheet("Sheet1")
	}

	for rowIdx, row := range rows {
		for colIdx, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			if err != nil {
				t.Fatalf("failed to compute cell name: %v", err)
			}
			if err := f.SetCellValue(sheetName, cell, value); err != nil {
				t.Fatalf("failed to set cell value: %v", err)
			}
		}
	}

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("failed to write xlsx to buffer: %v", err)
	}

	return buf.Bytes()
}

func TestParseXLSXRange_ReadsNamedSheet(t *testing.T) {
	data := buildTestXLSX(t, "TODO", [][]string{
		{"TÍTULO ORIGINAL", "SECCIÓN"},
		{"Film A", "Section A"},
		{"Film B", "Section B"},
	})

	values, err := parseXLSXRange(data, "TODO!A:ZZ")
	if err != nil {
		t.Fatalf("parseXLSXRange returned error: %v", err)
	}

	if len(values) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(values))
	}

	if values[0][0] != "TÍTULO ORIGINAL" || values[0][1] != "SECCIÓN" {
		t.Fatalf("unexpected header row: %v", values[0])
	}

	if values[1][0] != "Film A" || values[2][0] != "Film B" {
		t.Fatalf("unexpected data rows: %v", values[1:])
	}
}

func TestParseXLSXRange_SheetNotFound(t *testing.T) {
	data := buildTestXLSX(t, "TODO", [][]string{{"a"}})

	if _, err := parseXLSXRange(data, "MISSING!A:ZZ"); err == nil {
		t.Fatal("expected error for missing sheet, got nil")
	}
}

func TestParseXLSXRange_NoSheetSeparator(t *testing.T) {
	data := buildTestXLSX(t, "TODO", [][]string{{"a", "b"}})

	values, err := parseXLSXRange(data, "TODO")
	if err != nil {
		t.Fatalf("parseXLSXRange returned error: %v", err)
	}

	if len(values) != 1 || values[0][0] != "a" || values[0][1] != "b" {
		t.Fatalf("unexpected values: %v", values)
	}
}
