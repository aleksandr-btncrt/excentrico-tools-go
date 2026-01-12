package wordpress

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/models"
	"excentrico-tools-go/internal/services"
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

// UploadMediaToWordPress uploads optimized images to WordPress
func UploadMediaToWordPress(wordpressService *services.WordPressService, tursoService *services.TursoService, filmDir string, filmTitle string) ([]int, error) {
	l := logger.Get()
	op := l.StartOperation("upload_wordpress_media")
	
	filmID := utils.SanitizeFilename(filmTitle)
	op.WithFilm(filmID, filmTitle, "", "")
	op.WithContext("film_dir", filmDir)

	metadata := &models.WordPressMetadata{}
	err := tursoService.GetWordPressMetadata(filmID, metadata)
	if err != nil {
		if strings.Contains(err.Error(), "metadata not found") {
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

	var webFiles []string
	err = filepath.Walk(filmDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), "_web.jpg") {
			webFiles = append(webFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to find _web.jpg files: %v", err)
	}

	if len(webFiles) == 0 {
		log.Printf("No _web.jpg files found for '%s'", filmTitle)
		return []int{}, nil
	}

	log.Printf("Found %d _web.jpg files for '%s'", len(webFiles), filmTitle)

	existingImageMetadata := make(map[string]int)
	err = tursoService.GetWPImagesMetadata(filmID, &existingImageMetadata)
	if err != nil {
		if strings.Contains(err.Error(), "metadata not found") {
			log.Printf("No existing image metadata found for '%s', will start fresh", filmTitle)
		} else {
			log.Printf("Failed to load existing image metadata: %v", err)
		}
	} else {
		log.Printf("Loaded existing image metadata: %d images already uploaded", len(existingImageMetadata))
	}

	var uploadedMedia []map[string]any
	imageMetadataMap := make(map[string]int)

	for fileName, mediaID := range existingImageMetadata {
		imageMetadataMap[fileName] = mediaID
	}

	uploadedCount := 0
	skippedCount := 0
	failedUploads := 0

	for _, webFile := range webFiles {
		fileName := filepath.Base(webFile)

		if _, exists := existingImageMetadata[fileName]; exists {
			skippedCount++
			continue
		}

		title := strings.TrimSuffix(fileName, "_web.jpg")
		title = fmt.Sprintf("%s - %s", filmTitle, title)

		altText := fmt.Sprintf("Image from %s", filmTitle)

		uploadOp := l.StartOperation("upload_single_media")
		uploadOp.WithFilm(filmID, filmTitle, "", "")
		uploadOp.WithContext("file_name", fileName)
		uploadOp.WithContext("file_path", webFile)

		media, err := wordpressService.UploadMediaFromFile(webFile, title, altText)
		if err != nil {
			uploadOp.Fail(fmt.Sprintf("Failed to upload media %s", fileName), err)
			failedUploads++
			continue
		}

		uploadOp.WithWordPress(0, media.ID, "")
		uploadOp.WithContext("media_title", media.Title.String())
		uploadOp.Complete(fmt.Sprintf("Successfully uploaded media: %s", fileName))

		mediaInfo := map[string]any{
			"id":         media.ID,
			"title":      media.Title.String(),
			"source_url": media.SourceURL,
			"alt_text":   media.AltText,
			"file_path":  webFile,
			"post_id":    0, // Will be updated later when WordPress post is created
		}

		// If we have existing metadata, use the PostID
		if metadata != nil {
			mediaInfo["post_id"] = metadata.PostID
		}

		uploadedMedia = append(uploadedMedia, mediaInfo)

		imageMetadataMap[fileName] = media.ID
		uploadedCount++
	}

	if len(uploadedMedia) > 0 {
		if err := tursoService.SaveMetadata(filmID, "wordpress_media", uploadedMedia); err != nil {
			op.Warn(&logger.WideEvent{
				Message: "Failed to save media metadata",
			})
		}
	}

	if err := tursoService.SaveWPImagesMetadata(filmID, imageMetadataMap); err != nil {
		op.Warn(&logger.WideEvent{
			Message: "Failed to save image metadata",
		})
	}

	op.WithCounts(len(webFiles), len(webFiles), 0, skippedCount, uploadedCount, 0)
	op.WithContext("failed_uploads", failedUploads)
	op.Complete(fmt.Sprintf("Media upload completed: %d new uploads, %d skipped, %d total files", uploadedCount, skippedCount, len(webFiles)))

	var imageIds []int
	for _, mediaID := range imageMetadataMap {
		imageIds = append(imageIds, mediaID)
	}

	return imageIds, nil
}

// CreateOrUpdateWordPressProject creates or updates a WordPress project
func CreateOrUpdateWordPressProject(wordpressService *services.WordPressService, diviTemplateService *services.DiviTemplateService, tursoService *services.TursoService, filmDir string, filmData map[string]any, year string, imageIds []int, templateConfig *services.TemplateData) error {
	l := logger.Get()
	op := l.StartOperation("create_update_wordpress_project")
	
	filmTitle := "Untitled Film"
	if title, exists := filmData["TÍTULO ORIGINAL"]; exists && title != nil {
		filmTitle = title.(string)
	}

	filmID := utils.SanitizeFilename(filmTitle)
	
	section := ""
	if sec, exists := filmData["SECCIÓN"]; exists && sec != nil {
		section = sec.(string)
	}
	
	op.WithFilm(filmID, filmTitle, year, section)
	op.WithContext("film_dir", filmDir)
	op.WithContext("image_count", len(imageIds))

	var metadata *models.WordPressMetadata
	existingMetadata := &models.WordPressMetadata{}
	err := tursoService.GetWordPressMetadata(filmID, existingMetadata)
	if err != nil {
		if strings.Contains(err.Error(), "metadata not found") {
			op.WithContext("existing_metadata", false)
			metadata = nil
		} else {
			op.Fail("Error loading WordPress metadata", err)
			return err
		}
	} else {
		metadata = existingMetadata
		op.WithWordPress(metadata.PostID, 0, metadata.Slug)
		op.WithContext("existing_metadata", true)
	}

	slugParts := []string{}
	if filmTitle != "Untitled Film" {
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

	filmDataStruct := ConvertObjToFilmData(filmData)

	_, _ = diviTemplateService.GenerateCompleteTemplate(filmDataStruct, imageIds, wordpressService, tursoService, filmID, year, templateConfig)

	var categoryIDs []int
	if filmDataStruct.Seccion != "" {
		categoryNames := services.ParseCategoryString(filmDataStruct.Seccion)
		if len(categoryNames) > 0 {
			var err error
			categoryIDs, err = wordpressService.GetCategoryIDsByNames(year, categoryNames)
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
		Type:       "post",
		Slug:       CreateWordPressSlug(slugText),
		Categories: categoryIDs,
		Meta: map[string]any{
			"_et_pb_use_builder": "on",
		},
	}

	// Try to set a featured image from the uploaded media
	if len(imageIds) > 0 {
		if featuredID := selectFeaturedMediaID(imageIds, wordpressService); featuredID > 0 {
			post.FeaturedMedia = featuredID
		}
	}

	if metadata == nil {
		createOp := l.StartOperation("create_wordpress_post")
		createOp.WithFilm(filmID, filmTitle, year, section)
		createOp.WithContext("slug", CreateWordPressSlug(slugText))

		createdPost, err := wordpressService.CreatePost(post)
		if err != nil {
			createOp.Fail("Failed to create WordPress post", err)
			return fmt.Errorf("failed to create WordPress post: %v", err)
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
		updateOp := l.StartOperation("update_wordpress_post")
		updateOp.WithFilm(filmID, filmTitle, year, section)
		updateOp.WithWordPress(metadata.PostID, 0, metadata.Slug)

		post.ID = metadata.PostID
		updatedPost, err := wordpressService.UpdatePost(metadata.PostID, post)
		if err != nil {
			updateOp.Fail("Failed to update WordPress post", err)
			return fmt.Errorf("failed to update WordPress post: %v", err)
		}

		metadata.Title = updatedPost.Title.String()
		metadata.Slug = updatedPost.Slug
		metadata.Status = updatedPost.Status
		metadata.UpdatedAt = updatedPost.Modified

		updateOp.WithWordPress(updatedPost.ID, 0, updatedPost.Slug)
		updateOp.Complete(fmt.Sprintf("Updated WordPress post ID %d", updatedPost.ID))
	}

	if err := tursoService.SaveWordPressMetadata(filmID, metadata); err != nil {
		op.Fail("Failed to save WordPress metadata", err)
		return fmt.Errorf("failed to save WordPress metadata: %v", err)
	}

	// Update media metadata with the correct PostID if it was 0 initially
	if err := updateMediaMetadataWithPostID(tursoService, filmID, metadata.PostID); err != nil {
		op.Warn(&logger.WideEvent{
			Message: "Failed to update media metadata with PostID",
		})
		// Don't return error here as this is not critical
	}

	if err := diviTemplateService.SaveDiviTemplateToFile(filmDataStruct, imageIds, wordpressService, tursoService, filmID, filmDir, year, metadata.PostID, templateConfig ); err != nil {
		op.Fail("Failed to save Divi template to file", err)
		return fmt.Errorf("failed to save Divi template to file: %v", err)
	}

	op.WithWordPress(metadata.PostID, 0, metadata.Slug)
	op.Complete(fmt.Sprintf("Successfully created/updated WordPress project for '%s'", filmTitle))
	return nil
}

// selectFeaturedMediaID attempts to pick the most suitable featured image ID
// Preference order by media title/filename/alt text contains: poster, portada, cover
// Fallbacks to the first available image ID
func selectFeaturedMediaID(imageIds []int, wordpressService *services.WordPressService) int {
	if wordpressService == nil || len(imageIds) == 0 {
		return 0
	}

	keywords := []string{"poster", "portada", "cover"}

	// First pass: try to match preferred keywords
	for _, id := range imageIds {
		media, err := wordpressService.GetMedia(id)
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
func updateMediaMetadataWithPostID(tursoService *services.TursoService, filmID string, postID int) error {
	// Get existing media metadata
	var mediaMetadata []map[string]any
	err := tursoService.GetMetadata(filmID, "wordpress_media", &mediaMetadata)
	if err != nil {
		if strings.Contains(err.Error(), "metadata not found") {
			// No media metadata to update
			return nil
		}
		return err
	}

	// Update PostID for any media that has post_id = 0
	updated := false
	for _, media := range mediaMetadata {
		if postIDValue, exists := media["post_id"]; exists {
			if postIDInt, ok := postIDValue.(int); ok && postIDInt == 0 {
				media["post_id"] = postID
				updated = true
			}
		}
	}

	// Save updated metadata if any changes were made
	if updated {
		return tursoService.SaveMetadata(filmID, "wordpress_media", mediaMetadata)
	}

	return nil
}

// ConvertObjToFilmData converts a map[string]any to a FilmData struct
func ConvertObjToFilmData(obj map[string]any) *services.FilmData {
	filmData := &services.FilmData{
		AdditionalFields: make(map[string]string),
	}

	getString := func(key string) string {
		if value, exists := obj[key]; exists && value != nil {
			return value.(string)
		}
		return ""
	}

	filmData.TituloOriginal = getString("TÍTULO ORIGINAL")
	filmData.Direccion = getString("DIRECCIÓN")
	filmData.Pais = getString("PAIS")
	filmData.Ano = getString("AÑO")
	filmData.Duracion = getString("DURAC.")
	filmData.Edicion = getString("EDICIÓN")
	filmData.Seccion = getString("SECCIÓN")
	filmData.Tipo = getString("TIPO")
	filmData.SocialEtiquetas = getString("SOCIAL/ETIQUETAS")
	filmData.Idiomas = getString("Idioma(s) / Language(s)")
	filmData.RelacionAspectRatio = getString("Relación / Aspect Ratio (4:3, 16:9 u otro)")
	filmData.SinopsisExtendida = getString("Sinopsis extendida (máximo 70 palabras)")
	filmData.ExtendedSynopsis = getString("Extended synopsis (english)")
	filmData.SinopsisCompacta = getString("Sinopsis compacta  (máximo 10 palabras)")
	filmData.ShortSynopsis = getString("Short Synopsis (log line - Uso Pink Label)")
	filmData.NotasContenido = getString("Notas de contenido / Content notes (*)")
	filmData.NotaIntencion = getString("Nota de intención")
	filmData.Produccion = getString("Producción / Producer(s)")
	filmData.Guion = getString("Guión")
	filmData.CamaraFoto = getString("Cámara - Foto / Camera - Photography")
	filmData.ArteDiseno = getString("Arte - Diseño / Art/Design")
	filmData.SonidoMusica = getString("Sonido - Música / Sound - Music")
	filmData.EdicionCredits = getString("Edición / Editor(s)")
	filmData.Interpretes = getString("Intérpretes (especificar pronombres para subtítulos)/ Cast (please specify pronouns for subtitles)")
	filmData.OtrosCreditos = getString("Otros créditos / Other credits")
	filmData.FestivalesPremios = getString("Festivales y premios / Festivals & Awards")
	filmData.BioRealizadorxs = getString("Bio Realizadorxs / Filmaker's Bio (min 150 - max 1500 caracteres)")
	filmData.CorreoElectronico = getString("Correo electrónico / Email")
	filmData.Telefono = getString("Teléfono / Phone number")
	filmData.Enlaces = getString("ENLACES")
	filmData.WebExcentrico = getString("Web Excentrico")
	filmData.ImagenesBaja = getString("imágenes en baja")
	filmData.ObsSubtitulos = getString("Obs. Subtitulos")
	filmData.PublishedStatus = getString("Published Status")
	filmData.Categoria = getString("Categoría")
	filmData.MultiDir = getString("Multi Dir")

	knownFields := map[string]bool{
		"TÍTULO ORIGINAL": true, "DIRECCIÓN": true, "PAIS": true, "AÑO": true, "DURAC.": true, "EDICIÓN": true,
		"SECCIÓN": true, "TIPO": true, "SOCIAL/ETIQUETAS": true, "Idioma(s) / Language(s)": true,
		"Relación / Aspect Ratio (4:3, 16:9 u otro)": true, "Sinopsis extendida (máximo 70 palabras)": true,
		"Extended synopsis (english)": true, "Sinopsis compacta  (máximo 10 palabras)": true,
		"Short Synopsis (log line - Uso Pink Label)": true, "Notas de contenido / Content notes (*)": true,
		"Nota de intención": true, "Producción / Producer(s)": true, "Guión": true,
		"Cámara - Foto / Camera - Photography": true, "Arte - Diseño / Art/Design": true,
		"Sonido - Música / Sound - Music": true, "Edición / Editor(s)": true,
		"Intérpretes (especificar pronombres para subtítulos)/ Cast (please specify pronouns for subtitles)": true,
		"Otros créditos / Other credits": true, "Festivales y premios / Festivals & Awards": true,
		"Bio Realizadorxs / Filmaker's Bio (min 150 - max 1500 caracteres)": true,
		"Correo electrónico / Email":                                        true, "Teléfono / Phone number": true, "ENLACES": true,
		"Web Excentrico": true, "imágenes en baja": true, "Obs. Subtitulos": true,
		"Published Status": true, "Categoría": true, "Multi Dir": true,
	}

	for key, value := range obj {
		if !knownFields[key] && value != nil {
			if strValue, ok := value.(string); ok && strValue != "" {
				filmData.AdditionalFields[key] = strValue
			}
		}
	}

	return filmData
}
