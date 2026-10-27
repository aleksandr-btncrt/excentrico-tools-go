package film

import (
	"excentrico-tools-go/internal/drive"
	"excentrico-tools-go/internal/services"
	"excentrico-tools-go/internal/wordpress"
)

// The interfaces below are what the film processor consumes. They embed the
// narrow interfaces declared by the packages that run each operation, so
// the concrete services (wired in app.New) and the in-memory fakes used in
// tests are interchangeable, without import cycles.

// DriveClient lists and downloads Google Drive files.
type DriveClient interface {
	drive.Client
}

// ImageResizer writes the optimised *_web.jpg copy of an image.
type ImageResizer interface {
	drive.Resizer
}

// WordPressClient uploads/looks up media and creates/updates posts.
type WordPressClient interface {
	wordpress.Client
}

// MetadataStore persists per-film JSON metadata (Turso in production).
type MetadataStore interface {
	services.MetadataStore
}

var (
	_ DriveClient     = (*services.GoogleDriveService)(nil)
	_ ImageResizer    = (*services.ImageService)(nil)
	_ WordPressClient = (*services.WordPressService)(nil)
	_ MetadataStore   = (*services.TursoService)(nil)
)
