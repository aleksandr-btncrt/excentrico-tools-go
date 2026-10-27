package wordpress

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"excentrico-tools-go/internal/debug"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/models"
	"excentrico-tools-go/internal/services"
	"excentrico-tools-go/internal/sheet"
	"excentrico-tools-go/internal/utils"
)

// CreateWordPressSlug creates a URL-friendly slug from a title
func CreateWordPressSlug(title string) string {
	// Normalize accented characters to their ASCII equivalents
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	slug, _, _ := transform.String(t, title)

	slug = strings.ToLower(slug)

	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")

	slug = strings.Trim(slug, "-")

	if len(slug) > 50 {
		slug = slug[:50]
	}

	return slug
}

// isPathUnderDirectorDir returns true if path contains a path segment named "dir" (case-insensitive).
func isPathUnderDirectorDir(path string) bool {
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if strings.EqualFold(strings.TrimSpace(p), "dir") {
			return true
		}
	}
	return false
}

// normalizeDirectorCacheKey returns a stable key for director media cache.
func normalizeDirectorCacheKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// Client is the WordPress REST surface used by the operations here
// (*services.WordPressService in production).
type Client interface {
	services.MediaClient
	SearchMedia(ctx context.Context, search string) ([]*services.WordPressMedia, error)
	UploadMediaFromFile(ctx context.Context, filePath, title, altText string) (*services.WordPressMedia, error)
	GetCategoryIDsByNames(ctx context.Context, year string, categoryNames []string) ([]int, error)
	CreatePost(ctx context.Context, post *services.WordPressPost) (*services.WordPressPost, error)
	UpdatePost(ctx context.Context, postID int, post *services.WordPressPost) (*services.WordPressPost, error)
}

var _ Client = (*services.WordPressService)(nil)

// UploadMediaToWordPress uploads optimized images to WordPress. Director images (under a "Dir" folder)
// are uploaded with the director name as title and deduplicated via cache and WordPress search.
func UploadMediaToWordPress(ctx context.Context, wordpressService Client, tursoService services.MetadataStore, filmDir string, film *sheet.Film) ([]int, error) {
	filmTitle := film.Title()
	l := logger.Get()
	op := l.StartOperation("upload_wordpress_media")
	
	filmID := utils.SanitizeFilename(filmTitle)
	op.WithFilm(filmID, filmTitle, "", "")
	op.WithContext("film_dir", filmDir)

	metadata := &models.WordPressMetadata{}
	err := services.GetWordPressMetadata(tursoService, filmID, metadata)
	if err != nil {
		if errors.Is(err, services.ErrMetadataNotFound) {
			op.WithContext("existing_metadata", false)
			metadata = nil
		} else {
			op.Fail("Failed to load WordPress metadata", err)
			return nil, fmt.Errorf("failed to load WordPress metadata: %v", err)
		}
	} else {
		op.WithWordPress(metadata.PostID, 0, "")
		op.WithContext("existing_metadata", true)
	}

	type webFileEntry struct {
		path       string
		isDirector bool
	}
	var entries []webFileEntry
	err = filepath.Walk(filmDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), "_web.jpg") {
			entries = append(entries, webFileEntry{
				path:       path,
				isDirector: isPathUnderDirectorDir(path),
			})
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to find _web.jpg files: %v", err)
	}

	if len(entries) == 0 {
		op.Complete(fmt.Sprintf("No _web.jpg files found for '%s'", filmTitle))
		return []int{}, nil
	}

	op.WithContext("web_file_count", len(entries))

	directorNames := film.Directors()

	directorCache := make(map[string]services.DirectorMediaCacheEntry)
	if getCacheErr := services.GetDirectorMediaCache(tursoService, &directorCache); getCacheErr != nil && !errors.Is(getCacheErr, services.ErrMetadataNotFound) {
		l.Warnf("director_media_cache", "Failed to load director media cache: %v", getCacheErr)
	}

	existingImageMetadata := make(map[string]int)
	err = services.GetWPImagesMetadata(tursoService, filmID, &existingImageMetadata)
	if err != nil {
		if !errors.Is(err, services.ErrMetadataNotFound) {
			l.Warnf("upload_wordpress_media", "Failed to load existing image metadata for '%s': %v", filmTitle, err)
		}
	}
	op.WithContext("already_uploaded", len(existingImageMetadata))

	var uploadedMedia []models.MediaInfo
	postID := 0
	if metadata != nil {
		postID = metadata.PostID
	}
	imageMetadataMap := make(map[string]int)

	for fileName, mediaID := range existingImageMetadata {
		imageMetadataMap[fileName] = mediaID
	}

	uploadedCount := 0
	skippedCount := 0
	failedUploads := 0
	directorImageIndex := 0

	for _, entry := range entries {
		webFile := entry.path
		fileName := filepath.Base(webFile)

		// Skip if already uploaded (from DB or in a previous iteration — same base name in another dir)
		if _, exists := imageMetadataMap[fileName]; exists {
			skippedCount++
			continue
		}

		var media *services.WordPressMedia
		var mediaID int
		var sourceURL string
		titleForMeta := ""
		altTextForMeta := ""

		if entry.isDirector && len(directorNames) > 0 {
			directorName := directorNames[directorImageIndex%len(directorNames)]
			directorImageIndex++
			cacheKey := normalizeDirectorCacheKey(directorName)

			// 1) Check cache
			if cached, ok := directorCache[cacheKey]; ok {
				mediaID = cached.ID
				sourceURL = cached.SourceURL
				titleForMeta = directorName
				altTextForMeta = "Director: " + directorName
				skippedCount++
				debug.Printf("Using cached director image for '%s' (ID: %d)", directorName, mediaID)
			} else {
				// 2) Search WordPress for existing media with this director name
				searchResults, searchErr := wordpressService.SearchMedia(ctx, directorName)
				if searchErr == nil && len(searchResults) > 0 {
					// Prefer exact title match
					for _, m := range searchResults {
						if strings.EqualFold(strings.TrimSpace(m.Title.String()), strings.TrimSpace(directorName)) {
							mediaID = m.ID
							sourceURL = m.SourceURL
							titleForMeta = m.Title.String()
							altTextForMeta = m.AltText
							directorCache[cacheKey] = services.DirectorMediaCacheEntry{ID: m.ID, SourceURL: m.SourceURL}
							debug.Printf("Found existing WordPress media for director '%s' (ID: %d)", directorName, mediaID)
							break
						}
					}
					if mediaID == 0 && len(searchResults) > 0 {
						m := searchResults[0]
						mediaID = m.ID
						sourceURL = m.SourceURL
						titleForMeta = m.Title.String()
						altTextForMeta = m.AltText
						directorCache[cacheKey] = services.DirectorMediaCacheEntry{ID: m.ID, SourceURL: m.SourceURL}
						debug.Printf("Using first search result for director '%s' (ID: %d)", directorName, mediaID)
					}
				}
				// 3) If not in WP, upload with director name as title
				if mediaID == 0 {
					media, err = wordpressService.UploadMediaFromFile(ctx, webFile, directorName, "Director: "+directorName)
					if err != nil {
						uploadOp := l.StartOperation("upload_single_media")
						uploadOp.WithFilm(filmID, filmTitle, "", "")
						uploadOp.Fail(fmt.Sprintf("Failed to upload director media %s", fileName), err)
						failedUploads++
						continue
					}
					mediaID = media.ID
					sourceURL = media.SourceURL
					titleForMeta = media.Title.String()
					altTextForMeta = media.AltText
					directorCache[cacheKey] = services.DirectorMediaCacheEntry{ID: media.ID, SourceURL: media.SourceURL}
					if saveErr := services.SaveDirectorMediaCache(tursoService, directorCache); saveErr != nil {
						l.Warnf("director_media_cache", "Failed to save director media cache: %v", saveErr)
					}
					uploadedCount++
					l.StartOperation("upload_single_media").WithFilm(filmID, filmTitle, "", "").WithContext("file_name", fileName).Complete(fmt.Sprintf("Uploaded director image for '%s'", directorName))
				} else {
					skippedCount++
					if saveErr := services.SaveDirectorMediaCache(tursoService, directorCache); saveErr != nil {
						l.Warnf("director_media_cache", "Failed to save director media cache: %v", saveErr)
					}
				}
			}

			if mediaID != 0 {
				imageMetadataMap[fileName] = mediaID
				uploadedMedia = append(uploadedMedia, models.MediaInfo{
					ID:        mediaID,
					Title:     titleForMeta,
					SourceURL: sourceURL,
					AltText:   altTextForMeta,
					FilePath:  webFile,
					PostID:    postID,
				})
			}
			continue
		}

		// Non-director image: upload with film title prefix
		title := strings.TrimSuffix(fileName, "_web.jpg")
		title = fmt.Sprintf("%s - %s", filmTitle, title)
		altText := fmt.Sprintf("Image from %s", filmTitle)

		uploadOp := l.StartOperation("upload_single_media")
		uploadOp.WithFilm(filmID, filmTitle, "", "")
		uploadOp.WithContext("file_name", fileName)
		uploadOp.WithContext("file_path", webFile)

		media, err = wordpressService.UploadMediaFromFile(ctx, webFile, title, altText)
		if err != nil {
			uploadOp.Fail(fmt.Sprintf("Failed to upload media %s", fileName), err)
			failedUploads++
			continue
		}

		uploadOp.WithWordPress(0, media.ID, "")
		uploadOp.WithContext("media_title", media.Title.String())
		uploadOp.Complete(fmt.Sprintf("Successfully uploaded media: %s", fileName))

		uploadedMedia = append(uploadedMedia, models.MediaInfo{
			ID:        media.ID,
			Title:     media.Title.String(),
			SourceURL: media.SourceURL,
			AltText:   media.AltText,
			FilePath:  webFile,
			PostID:    postID,
		})
		imageMetadataMap[fileName] = media.ID
		uploadedCount++
	}

	if len(uploadedMedia) > 0 {
		if err := tursoService.SaveMetadata(filmID, services.MetadataWordPressMedia, uploadedMedia); err != nil {
			op.Warn(&logger.WideEvent{
				Message: "Failed to save media metadata",
			})
		}
	}

	if err := services.SaveWPImagesMetadata(tursoService, filmID, imageMetadataMap); err != nil {
		op.Warn(&logger.WideEvent{
			Message: "Failed to save image metadata",
		})
	}

	op.WithCounts(len(entries), len(entries), 0, skippedCount, uploadedCount, 0)
	op.WithContext("failed_uploads", failedUploads)
	op.Complete(fmt.Sprintf("Media upload completed: %d new uploads, %d skipped, %d total files", uploadedCount, skippedCount, len(entries)))

	var imageIds []int
	for _, mediaID := range imageMetadataMap {
		imageIds = append(imageIds, mediaID)
	}

	return imageIds, nil
}

// CreateOrUpdateWordPressProject creates or updates the film's WordPress
// project post and returns its post ID.
func CreateOrUpdateWordPressProject(ctx context.Context, wordpressService Client, diviTemplateService *services.DiviTemplateService, tursoService services.MetadataStore, filmDir string, film *sheet.Film, year string, imageIds []int, templateConfig services.TemplateData) (int, error) {
	l := logger.Get()
	op := l.StartOperation("create_update_wordpress_project")

	// Same identity as UploadMediaToWordPress and the film directory, so the
	// stored metadata is found again (previously an untitled row used
	// "Untitled Film" here and "unnamed_film" elsewhere).
	filmTitle := film.Title()
	filmID := utils.SanitizeFilename(filmTitle)
	section := film.Seccion

	op.WithFilm(filmID, filmTitle, year, section)
	op.WithContext("film_dir", filmDir)
	op.WithContext("image_count", len(imageIds))

	var metadata *models.WordPressMetadata
	existingMetadata := &models.WordPressMetadata{}
	err := services.GetWordPressMetadata(tursoService, filmID, existingMetadata)
	if err != nil {
		if errors.Is(err, services.ErrMetadataNotFound) {
			op.WithContext("existing_metadata", false)
			metadata = nil
		} else {
			op.Fail("Error loading WordPress metadata", err)
			return 0, err
		}
	} else {
		metadata = existingMetadata
		op.WithWordPress(metadata.PostID, 0, metadata.Slug)
		op.WithContext("existing_metadata", true)
	}

	slugParts := []string{}
	if film.HasTitle() {
		slugParts = append(slugParts, filmTitle)
	}
	if section != "" {
		slugParts = append(slugParts, section)
	}
	if year != "" {
		slugParts = append(slugParts, year)
	}

	slugText := strings.Join(slugParts, " ")
	if slugText == "" {
		slugText = "untitled-film"
	}

	_, _ = diviTemplateService.GenerateCompleteTemplate(ctx, film, imageIds, wordpressService, tursoService, filmID, year, templateConfig)

	var categoryIDs []int
	if film.Seccion != "" {
		categoryNames := services.ParseCategoryString(film.Seccion)
		if len(categoryNames) > 0 {
			var err error
			categoryIDs, err = wordpressService.GetCategoryIDsByNames(ctx, year, categoryNames)
			if err != nil {
				op.Warn(&logger.WideEvent{
					Message: fmt.Sprintf("Failed to get category IDs for film '%s'", filmTitle),
				})
			} else {
				op.WithContext("category_count", len(categoryIDs))
			}
		}
	}

	post := &services.WordPressPost{
		Title:      services.WordPressRenderedField{Rendered: filmTitle},
		Status:     "draft",
		Slug:       CreateWordPressSlug(slugText),
		Categories: categoryIDs,
		Meta: map[string]any{
			"_et_pb_use_builder": "on",
		},
	}

	// Try to set a featured image from the uploaded media
	if len(imageIds) > 0 {
		if featuredID := selectFeaturedMediaID(ctx, imageIds, wordpressService); featuredID > 0 {
			post.FeaturedMedia = featuredID
		}
	}

	if metadata == nil {
		createOp := l.StartOperation("create_wordpress_post").Important()
		createOp.WithFilm(filmID, filmTitle, year, section)
		createOp.WithContext("slug", CreateWordPressSlug(slugText))

		createdPost, err := wordpressService.CreatePost(ctx, post)
		if err != nil {
			createOp.Fail("Failed to create WordPress post", err)
			return 0, fmt.Errorf("failed to create WordPress post: %v", err)
		}

		metadata = &models.WordPressMetadata{
			PostID:    createdPost.ID,
			Title:     createdPost.Title.String(),
			Slug:      createdPost.Slug,
			Status:    createdPost.Status,
			CreatedAt: createdPost.Date,
			UpdatedAt: createdPost.Modified,
		}

		createOp.WithWordPress(createdPost.ID, 0, createdPost.Slug)
		createOp.Complete(fmt.Sprintf("Created WordPress post ID %d", createdPost.ID))
	} else {
		updateOp := l.StartOperation("update_wordpress_post").Important()
		updateOp.WithFilm(filmID, filmTitle, year, section)
		updateOp.WithWordPress(metadata.PostID, 0, metadata.Slug)

		post.ID = metadata.PostID
		updatedPost, err := wordpressService.UpdatePost(ctx, metadata.PostID, post)
		if err != nil {
			updateOp.Fail("Failed to update WordPress post", err)
			return 0, fmt.Errorf("failed to update WordPress post: %v", err)
		}

		metadata.Title = updatedPost.Title.String()
		metadata.Slug = updatedPost.Slug
		metadata.Status = updatedPost.Status
		metadata.UpdatedAt = updatedPost.Modified

		updateOp.WithWordPress(updatedPost.ID, 0, updatedPost.Slug)
		updateOp.Complete(fmt.Sprintf("Updated WordPress post ID %d", updatedPost.ID))
	}

	if err := services.SaveWordPressMetadata(tursoService, filmID, metadata); err != nil {
		op.Fail("Failed to save WordPress metadata", err)
		return 0, fmt.Errorf("failed to save WordPress metadata: %v", err)
	}

	// Update media metadata with the correct PostID if it was 0 initially
	if err := updateMediaMetadataWithPostID(tursoService, filmID, metadata.PostID); err != nil {
		op.Warn(&logger.WideEvent{
			Message: "Failed to update media metadata with PostID",
		})
		// Don't return error here as this is not critical
	}

	if err := diviTemplateService.SaveDiviTemplateToFile(ctx, film, imageIds, wordpressService, tursoService, filmID, filmDir, year, metadata.PostID, templateConfig); err != nil {
		op.Fail("Failed to save Divi template to file", err)
		return 0, fmt.Errorf("failed to save Divi template to file: %v", err)
	}

	op.WithWordPress(metadata.PostID, 0, metadata.Slug)
	op.Complete(fmt.Sprintf("Successfully created/updated WordPress project for '%s'", filmTitle))
	return metadata.PostID, nil
}

// selectFeaturedMediaID attempts to pick the most suitable featured image ID
// Preference order by media title/filename/alt text contains: poster, portada, cover
// Fallbacks to the first available image ID
func selectFeaturedMediaID(ctx context.Context, imageIds []int, wordpressService services.MediaClient) int {
	if wordpressService == nil || len(imageIds) == 0 {
		return 0
	}

	keywords := []string{"poster", "portada", "cover"}

	// First pass: try to match preferred keywords
	for _, id := range imageIds {
		media, err := wordpressService.GetMedia(ctx, id)
		if err != nil {
			continue
		}

		title := strings.ToLower(media.Title.String())
		alt := strings.ToLower(media.AltText)
		filename := ""
		if media.SourceURL != "" {
			parts := strings.Split(media.SourceURL, "/")
			if len(parts) > 0 {
				filename = parts[len(parts)-1]
				if dot := strings.LastIndex(filename, "."); dot != -1 {
					filename = filename[:dot]
				}
				filename = strings.ToLower(filename)
			}
		}

		for _, kw := range keywords {
			if strings.Contains(title, kw) || strings.Contains(filename, kw) || strings.Contains(alt, kw) {
				return id
			}
		}
	}

	// Second pass: just return the first available
	return imageIds[0]
}

// updateMediaMetadataWithPostID updates media metadata with the correct PostID
func updateMediaMetadataWithPostID(tursoService services.MetadataStore, filmID string, postID int) error {
	// Get existing media metadata
	var mediaMetadata []models.MediaInfo
	err := tursoService.GetMetadata(filmID, services.MetadataWordPressMedia, &mediaMetadata)
	if err != nil {
		if errors.Is(err, services.ErrMetadataNotFound) {
			// No media metadata to update
			return nil
		}
		return err
	}

	// Update PostID for any media that has post_id = 0. (Previously this
	// asserted .(int) on a JSON-decoded value, which is always float64, so the
	// update never happened.)
	updated := false
	for i := range mediaMetadata {
		if mediaMetadata[i].PostID == 0 {
			mediaMetadata[i].PostID = postID
			updated = true
		}
	}

	// Save updated metadata if any changes were made
	if updated {
		return tursoService.SaveMetadata(filmID, services.MetadataWordPressMedia, mediaMetadata)
	}

	return nil
}
