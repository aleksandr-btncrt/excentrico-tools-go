package models

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMediaInfoRoundTrip(t *testing.T) {
	in := []MediaInfo{{ID: 42, Title: "T", SourceURL: "https://x/y_web.jpg", AltText: "alt", FilePath: "films/a/y_web.jpg", PostID: 0}}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out []MediaInfo
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip mismatch: %+v vs %+v", in, out)
	}
}

// Data already stored in Turso was written from map[string]any; it must
// decode into MediaInfo unchanged.
func TestMediaInfoDecodesLegacyMapJSON(t *testing.T) {
	legacy := []map[string]any{{
		"id": 7, "title": "Film - still", "source_url": "https://x/s_web.jpg",
		"alt_text": "Image from Film", "file_path": "films/Film/Stills/s_web.jpg", "post_id": 0,
	}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var out []MediaInfo
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	want := MediaInfo{ID: 7, Title: "Film - still", SourceURL: "https://x/s_web.jpg", AltText: "Image from Film", FilePath: "films/Film/Stills/s_web.jpg"}
	if len(out) != 1 || out[0] != want {
		t.Errorf("got %+v, want %+v", out, want)
	}
}
