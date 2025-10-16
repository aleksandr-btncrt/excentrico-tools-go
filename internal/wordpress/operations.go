package wordpress

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"excentrico-tools-go/internal/models"
	"excentrico-tools-go/internal/services"
	"excentrico-tools-go/internal/utils"
)

// CreateWordPressSlug creates a URL-friendly slug from a title
func CreateWordPressSlug(title string) string {
	slug := strings.ToLower(title)

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
	filmID := utils.SanitizeFilename(filmTitle)

	metadata := &models.WordPressMetadata{}
	err := tursoService.GetWordPressMetadata(filmID, metadata)
	if err != nil {
		if strings.Contains(err.Error(), "metadata not found") {
			log.Printf("No existing WordPress metadata found for '%s', will proceed with media upload", filmTitle)
			metadata = nil
		} else {
			return nil, fmt.Errorf("failed to load WordPress metadata: %v", err)
		}
	} else {
		log.Printf("Loaded existing WordPress metadata for '%s' (Post ID: %d)", filmTitle, metadata.PostID)
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

	for _, webFile := range webFiles {
		fileName := filepath.Base(webFile)

		if existingID, exists := existingImageMetadata[fileName]; exists {
			log.Printf("Skipping already uploaded image: %s (WordPress ID: %d)", fileName, existingID)
			skippedCount++
			continue
		}

		title := strings.TrimSuffix(fileName, "_web.jpg")
		title = fmt.Sprintf("%s - %s", filmTitle, title)

		altText := fmt.Sprintf("Image from %s", filmTitle)

		log.Printf("Uploading media: %s", webFile)

		media, err := wordpressService.UploadMediaFromFile(webFile, title, altText)
		if err != nil {
			log.Printf("Failed to upload media %s: %v", webFile, err)
			continue
		}

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

		log.Printf("Successfully uploaded media: %s (ID: %d)", media.Title.String(), media.ID)
	}

	if len(uploadedMedia) > 0 {
		if err := tursoService.SaveMetadata(filmID, "wordpress_media", uploadedMedia); err != nil {
			log.Printf("Failed to save media metadata: %v", err)
		} else {
			log.Printf("Saved media metadata for '%s' (%d files) to database", filmTitle, len(uploadedMedia))
		}
	}

	if err := tursoService.SaveWPImagesMetadata(filmID, imageMetadataMap); err != nil {
		log.Printf("Failed to save image metadata: %v", err)
	} else {
		log.Printf("Saved image metadata mapping for '%s' (%d files) to database", filmTitle, len(imageMetadataMap))
	}

	log.Printf("Media upload completed for '%s': %d new uploads, %d skipped, %d total files", filmTitle, uploadedCount, skippedCount, len(webFiles))

	var imageIds []int
	for _, mediaID := range imageMetadataMap {
		imageIds = append(imageIds, mediaID)
	}

	return imageIds, nil
}

// CreateOrUpdateWordPressProject creates or updates a WordPress project
func CreateOrUpdateWordPressProject(wordpressService *services.WordPressService, diviTemplateService *services.DiviTemplateService, tursoService *services.TursoService, filmDir string, filmData map[string]any, year string, imageIds []int, templateConfig *services.TemplateData) error {
	filmTitle := "Untitled Film"
	if title, exists := filmData["TÍTULO ORIGINAL"]; exists && title != nil {
		filmTitle = title.(string)
	}

	filmID := utils.SanitizeFilename(filmTitle)

	var metadata *models.WordPressMetadata
	existingMetadata := &models.WordPressMetadata{}
	err := tursoService.GetWordPressMetadata(filmID, existingMetadata)
	if err != nil {
		if strings.Contains(err.Error(), "metadata not found") {
			log.Printf("No existing WordPress metadata found for '%s', will create new", filmTitle)
			metadata = nil
		} else {
			log.Printf("Error loading WordPress metadata: %v", err)
			return err
		}
	} else {
		metadata = existingMetadata
		log.Printf("Loaded existing WordPress metadata for '%s' (Post ID: %d)", filmTitle, metadata.PostID)
	}

	section := ""
	if sec, exists := filmData["SECCIÓN"]; exists && sec != nil {
		section = sec.(string)
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

	_, _ = diviTemplateService.GenerateCompleteTemplate(filmDataStruct, imageIds, wordpressService, year, templateConfig)

	var categoryIDs []int
	if filmDataStruct.Categoria != "" {
		categoryNames := services.ParseCategoryString(filmDataStruct.Categoria)
		if len(categoryNames) > 0 {
			var err error
			categoryIDs, err = wordpressService.GetCategoryIDsByNames(categoryNames)
			if err != nil {
				log.Printf("Warning: Failed to get category IDs for film '%s': %v", filmTitle, err)
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

	if metadata == nil {
		log.Printf("Creating new WordPress project for '%s'", filmTitle)

		createdPost, err := wordpressService.CreatePost(post)
		if err != nil {
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

		log.Printf("Created WordPress post ID %d for '%s'", createdPost.ID, filmTitle)
	} else {
		log.Printf("Updating WordPress project for '%s' (ID: %d)", filmTitle, metadata.PostID)

		post.ID = metadata.PostID
		updatedPost, err := wordpressService.UpdatePost(metadata.PostID, post)
		if err != nil {
			return fmt.Errorf("failed to update WordPress post: %v", err)
		}

		metadata.Title = updatedPost.Title.String()
		metadata.Slug = updatedPost.Slug
		metadata.Status = updatedPost.Status
		metadata.UpdatedAt = updatedPost.Modified
	}

	if err := tursoService.SaveWordPressMetadata(filmID, metadata); err != nil {
		return fmt.Errorf("failed to save WordPress metadata: %v", err)
	}

	// Update media metadata with the correct PostID if it was 0 initially
	if err := updateMediaMetadataWithPostID(tursoService, filmID, metadata.PostID); err != nil {
		log.Printf("Failed to update media metadata with PostID: %v", err)
		// Don't return error here as this is not critical
	}

	if err := diviTemplateService.SaveDiviTemplateToFile(filmDataStruct, imageIds, wordpressService, filmDir, year, metadata.PostID, templateConfig ); err != nil {
		return fmt.Errorf("failed to save Divi template to file: %v", err)
	}

	return nil
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
