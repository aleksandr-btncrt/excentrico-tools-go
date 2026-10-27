package services

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"excentrico-tools-go/internal/models"
)

// mapStore is an in-memory MetadataStore keyed by film ID and type.
type mapStore map[string][]byte

func (m mapStore) SaveMetadata(filmID, metadataType string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	m[filmID+"/"+metadataType] = b
	return nil
}

func (m mapStore) GetMetadata(filmID, metadataType string, dest any) error {
	b, ok := m[filmID+"/"+metadataType]
	if !ok {
		return fmt.Errorf("%s/%s: %w", filmID, metadataType, ErrMetadataNotFound)
	}
	return json.Unmarshal(b, dest)
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"1:30:15", "90´15"},
		{"0:05:00", "5´00"},
		{" 2 : 01 : 09 ", "121´09"},
		{"45:10", "45´10"},
		{"x:y:z", "0´0"},
		{"90", "0´0"},
		{"", "0´0"},
	}
	for _, tt := range tests {
		if got := parseDuration(tt.in); got != tt.want {
			t.Errorf("parseDuration(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFilterStillsImages(t *testing.T) {
	s := NewDiviTemplateService()
	store := mapStore{}
	_ = store.SaveMetadata("film", MetadataWordPressMedia, []models.MediaInfo{
		{ID: 1, FilePath: "films/film/Stills/still_one_web.jpg"},
		{ID: 2, FilePath: "films/film/Background/bg_web.jpg"},
		{ID: 3, FilePath: "films/film/Stills/Still_Two_web.jpg"},
		{ID: 4, FilePath: "films/film/Dir/jane_web.jpg"},
	})
	_ = SaveDriveFilesMetadata(store, "film", []*models.FileWithPath{
		{Name: "still_one.jpg", FolderName: "Stills"},
		{Name: "bg.png", FolderName: "Background"},
		{Name: "still_two.JPG", FolderName: "STILLS"},
		{Name: "jane.jpg", FolderName: "Dir"},
	})

	got := s.filterStillsImages([]int{4, 3, 2, 1, 99}, store, "film")
	if want := []int{3, 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("filterStillsImages = %v, want %v (input order, stills only)", got, want)
	}

	for name, tc := range map[string]struct {
		store  MetadataStore
		ids    []int
		filmID string
	}{
		"nil store":      {nil, []int{1}, "film"},
		"no ids":         {store, nil, "film"},
		"no film id":     {store, []int{1}, ""},
		"unknown film":   {store, []int{1}, "other"},
		"no drive files": {mapStore{"film/" + MetadataWordPressMedia: store["film/"+MetadataWordPressMedia]}, []int{1}, "film"},
	} {
		if got := s.filterStillsImages(tc.ids, tc.store, tc.filmID); len(got) != 0 {
			t.Errorf("%s: filterStillsImages = %v, want empty", name, got)
		}
	}
}
