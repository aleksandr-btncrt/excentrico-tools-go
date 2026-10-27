package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"excentrico-tools-go/internal/debug"
	"excentrico-tools-go/internal/logger"
	"excentrico-tools-go/internal/models"
	"excentrico-tools-go/internal/sheet"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

// Template constants - Colors
const (
	ColorPrimary       = "#24A68E" // Teal
	ColorSecondary     = "#E6F543" // Lime green
	ColorDark          = "#31045C" // Dark purple
	ColorYellow        = "#FFFC4F" // Yellow
	ColorCoral         = "#FF9582" // Coral
	ColorLightGreen    = "#91F580" // Light green
	ColorWhite         = "#ffffff" // White
	ColorBody          = "#333333" // Body text
	ColorPink          = "#FFDBFF" // Pink
	ColorGradientStart = "#3333cc" // Gradient start
	ColorGradientEnd   = "#6633cc" // Gradient end
)

// Template constants - URLs
const (
	URLHeaderBackground = "https://excentricofest.com/wp-content/uploads/2024/11/imagen2025.jpg"
	URLFacebook         = "https://www.facebook.com/excentrico.fest/"
	URLInstagram        = "https://www.instagram.com/excentrico.fest/?hl=es"
	URLTwitter          = "https://twitter.com/excentricofest"
	EmailContact        = "hola@excentricofest.com"
)

// Template constants - Builder version
const (
	BuilderVersion = "4.27.4"
)

// Template constants - Font settings
const (
	FontBold      = "|600|||||||"
	FontBoldCaps  = "|600||on|||||"
	FontExtraBold = "|800|||||||"
)

// Template constants - Common settings
const (
	ModulePresetDefault = "_module_preset=\"default\""
	GlobalColorsInfo    = "global_colors_info=\"{}\""
	BoxShadowPreset3    = "box_shadow_style=\"preset3\""
	CustomButtonOn      = "custom_button=\"on\""
	BackgroundColorOff  = "background_enable_color=\"off\""
	BackgroundColorOn   = "background_enable_color=\"on\""
)

// Template constants - Padding/Margin
const (
	PaddingStandard = "3%|3%|3.6%|3%|false|false"
	PaddingDirector = "3.3%|3%|3.6%|3%|false|false"
	MarginStandard  = "6%||||false|false"
	PaddingNotes    = "1%||1%|2%|false|false"
)

// DiviTemplateService renders the Divi shortcodes and divi_template.json.
type DiviTemplateService struct {
	// EmbedImages downloads every image and embeds it as base64 in
	// divi_template.json (-embed-images). When false, "encoded" is empty and
	// Divi resolves images from their "url"/"id".
	EmbedImages bool
}

// NewDiviTemplateService returns a service that embeds images, which is the
// behaviour divi_template.json imports have been validated with.
func NewDiviTemplateService() *DiviTemplateService {
	return &DiviTemplateService{EmbedImages: true}
}

func escapeHtml(text string) string {
	return html.EscapeString(text)
}

func parseDuration(duration string) string {
	parts := strings.Split(duration, ":")

	if len(parts) == 3 {
		hours := strings.TrimSpace(parts[0])
		mins := strings.TrimSpace(parts[1])
		seconds := strings.TrimSpace(parts[2])

		hoursInt := 0
		minsInt := 0

		if h, err := fmt.Sscanf(hours, "%d", &hoursInt); err == nil && h == 1 {
			if m, err := fmt.Sscanf(mins, "%d", &minsInt); err == nil && m == 1 {
				totalMinutes := hoursInt*60 + minsInt
				return fmt.Sprintf("%d´%s", totalMinutes, seconds)
			}
		}
	} else if len(parts) == 2 {
		minutes := strings.TrimSpace(parts[0])
		seconds := strings.TrimSpace(parts[1])
		return fmt.Sprintf("%s´%s", minutes, seconds)
	}

	return "0´0"
}

type DirectorInfo struct {
	Name     string `json:"name"`
	ImageURL string `json:"image_url,omitempty"`
	Bio      string `json:"bio,omitempty"`
}

type DiviFilmTemplate struct {
	Title           string         `json:"title"`
	Country         string         `json:"country"`
	Year            string         `json:"year"`
	Duration        string         `json:"duration"`
	BackgroundImage string         `json:"background_image,omitempty"`
	FeaturedImage   string         `json:"featured_image,omitempty"`
	Directors       []DirectorInfo `json:"directors"`
	Synopsis        string         `json:"synopsis"`
	ContentNotes    string         `json:"content_notes"`
	Credits         Credits        `json:"credits"`
	ImageGalleryIds []int          `json:"image_gallery_ids"`
	GalleryMediaIds string         `json:"gallery_media_ids"`
}

type Credits struct {
	Production   string `json:"production,omitempty"`
	Script       string `json:"script,omitempty"`
	Photography  string `json:"photography,omitempty"`
	ArtDesign    string `json:"art_design,omitempty"`
	SoundMusic   string `json:"sound_music,omitempty"`
	Editing      string `json:"editing,omitempty"`
	Cast         string `json:"cast,omitempty"`
	OtherCredits string `json:"other_credits,omitempty"`
}

// MediaClient is what the media selectors and the Divi template file need
// from WordPress: one batched media lookup per film and image downloads
// (for -embed-images).
type MediaClient interface {
	FetchMedia(ctx context.Context, ids []int) (*MediaCache, error)
	Download(ctx context.Context, url string) ([]byte, error)
}

var _ MediaClient = (*WordPressService)(nil)

// GenerateDiviTemplateData builds the template data for a film. Media
// (director, background and gallery images) is looked up in cache.
func (s *DiviTemplateService) GenerateDiviTemplateData(filmData *sheet.Film, imageIds []int, cache *MediaCache, tursoService MetadataStore, filmID string) *DiviFilmTemplate {
	// Directors with bio information ("Multi Dir" = SI splits DIRECCIÓN).
	var directors []DirectorInfo
	for _, name := range filmData.Directors() {
		directors = append(directors, DirectorInfo{
			Name:     name,
			Bio:      filmData.BioRealizadorxs,
			ImageURL: s.findDirectorImage(name, imageIds, cache),
		})
	}

	// Format duration
	formattedDuration := ""
	if filmData.Duracion != "" {
		formattedDuration = parseDuration(filmData.Duracion)
	}

	// Build credits structure
	credits := Credits{
		Production:   filmData.Produccion,
		Script:       filmData.Guion,
		Photography:  filmData.CamaraFoto,
		ArtDesign:    filmData.ArteDiseno,
		SoundMusic:   filmData.SonidoMusica,
		Editing:      filmData.EdicionCredits,
		Cast:         filmData.Interpretes,
		OtherCredits: filmData.OtrosCreditos,
	}

	// Filter to only include stills images for the gallery
	stillsImageIds := s.filterStillsImages(imageIds, tursoService, filmID)

	// Prepare gallery media IDs as comma-separated string
	var galleryIds []string
	for _, id := range stillsImageIds {
		galleryIds = append(galleryIds, fmt.Sprintf("%d", id))
	}
	galleryMediaIds := strings.Join(galleryIds, ",")

	var backgroundImage string
	// Try to infer a background image from uploaded media
	if url := s.selectBackgroundImageURL(imageIds, cache); url != "" {
		backgroundImage = url
	}
	
	// Create the complete template data
	template := &DiviFilmTemplate{
		Title:           filmData.TituloOriginal,
		Country:         filmData.Pais,
		Year:            filmData.Ano,
		Duration:        formattedDuration,
		BackgroundImage: backgroundImage,
		Directors:       directors,
		Synopsis:        filmData.SinopsisExtendida,
		ContentNotes:    filmData.NotasContenido,
		Credits:         credits,
		ImageGalleryIds: stillsImageIds,
		GalleryMediaIds: galleryMediaIds,
	}

	return template
}

// filterStillsImages filters imageIds to only include images from the Stills folder
// Uses drive metadata to determine which images are from the Stills folder
func (s *DiviTemplateService) filterStillsImages(imageIds []int, tursoService MetadataStore, filmID string) []int {
	if tursoService == nil || len(imageIds) == 0 || filmID == "" {
		return []int{}
	}

	// Get WordPress media metadata to map image IDs to file paths
	var wpMediaMetadata []models.MediaInfo
	err := tursoService.GetMetadata(filmID, MetadataWordPressMedia, &wpMediaMetadata)
	if err != nil {
		// If no WordPress media metadata, return empty
		return []int{}
	}

	// Create a map from WordPress media ID to file path
	mediaIDToFilePath := make(map[int]string)
	for _, media := range wpMediaMetadata {
		if media.ID != 0 && media.FilePath != "" {
			mediaIDToFilePath[media.ID] = media.FilePath
		}
	}

	// Get drive files metadata
	var driveFiles []*models.FileWithPath
	err = GetDriveFilesMetadata(tursoService, filmID, &driveFiles)
	if err != nil {
		// If no drive metadata, return empty
		return []int{}
	}

	// Create a map from filename (without _web.jpg) to FolderName
	filenameToFolder := make(map[string]string)
	for _, file := range driveFiles {
		// Remove _web.jpg suffix if present, and normalize
		filename := strings.TrimSuffix(strings.ToLower(file.Name), "_web.jpg")
		// Also try without any extension
		if dot := strings.LastIndex(filename, "."); dot != -1 {
			filenameWithoutExt := filename[:dot]
			filenameToFolder[filenameWithoutExt] = strings.ToLower(file.FolderName)
		}
		filenameToFolder[filename] = strings.ToLower(file.FolderName)
	}

	// Filter imageIds to only include those from Stills folder
	var stillsIds []int
	for _, id := range imageIds {
		filePath, exists := mediaIDToFilePath[id]
		if !exists {
			continue
		}

		// Extract filename from file path
		fileName := filepath.Base(filePath)
		// Remove _web.jpg suffix
		fileName = strings.TrimSuffix(strings.ToLower(fileName), "_web.jpg")
		// Remove extension
		if dot := strings.LastIndex(fileName, "."); dot != -1 {
			fileName = fileName[:dot]
		}

		// Check if this file is from Stills folder
		if folderName, exists := filenameToFolder[fileName]; exists {
			if folderName == "stills" {
				stillsIds = append(stillsIds, id)
			}
		}
	}

	return stillsIds
}

// selectBackgroundImageURL attempts to pick a background image URL from media
// Preference order by media title/filename/alt text contains: background, header, fondo, bg
// Falls back to the first media URL if any
func (s *DiviTemplateService) selectBackgroundImageURL(imageIds []int, cache *MediaCache) string {
	if len(imageIds) == 0 {
		return ""
	}

	keywords := []string{"background", "header", "fondo", "bg"}

	// First pass: try to match keywords
	for _, id := range imageIds {
		media, ok := cache.Get(id)
		if !ok {
			continue
		}
		if MediaMatchesAny(media, keywords) {
			return media.SourceURL
		}
	}

	// Second pass: fallback to first media URL
	if first, ok := cache.Get(imageIds[0]); ok {
		return first.SourceURL
	}
	return ""
}

// MediaMatchesAny reports whether the media title, file name or alt text
// contains one of keywords (case-insensitive; keywords are lower case).
func MediaMatchesAny(media *WordPressMedia, keywords []string) bool {
	title := strings.ToLower(media.Title.String())
	alt := strings.ToLower(media.AltText)
	filename := MediaFilenameStem(media.SourceURL)
	for _, kw := range keywords {
		if strings.Contains(title, kw) || strings.Contains(filename, kw) || strings.Contains(alt, kw) {
			return true
		}
	}
	return false
}

func (s *DiviTemplateService) GenerateDiviShortcodeTemplate(templateData *DiviFilmTemplate, year string, templateConfig TemplateData) string {
	// Use the factory function to create a standard template composition
	composer := s.CreateStandardFilmTemplate(templateData, year, templateConfig)
	return composer.Compose()
}

// GenerateCompleteTemplate builds the template data and its Divi shortcodes.
func (s *DiviTemplateService) GenerateCompleteTemplate(filmData *sheet.Film, imageIds []int, cache *MediaCache, tursoService MetadataStore, filmID string, year string, templateConfig TemplateData) (*DiviFilmTemplate, string) {
	templateData := s.GenerateDiviTemplateData(filmData, imageIds, cache, tursoService, filmID)

	shortcodes := s.GenerateDiviShortcodeTemplate(templateData, year, templateConfig)

	return templateData, shortcodes
}

// Factory function to create a standard film template with all components
func (s *DiviTemplateService) CreateStandardFilmTemplate(templateData *DiviFilmTemplate, year string, templateConfig TemplateData) *DiviTemplateComposer {
	// Create subhead with country, year, and duration
	subhead := fmt.Sprintf("%s · %s · %s", templateData.Country, templateData.Year, templateData.Duration)

	// Build button text
	buttonText := "convocatoria"
	if year != "" {
		buttonText = fmt.Sprintf("convocatoria %s", year)
	}

	// Create reusable components
	creditsComponent := &CreditsComponent{
		Directors: templateData.Directors,
		Credits:   templateData.Credits,
	}

	contentNotesComponent := &ContentNotesComponent{
		ContentNotes: templateData.ContentNotes,
		NdcProps:     templateConfig.Ndc,
	}

	directorComponent := &DirectorComponent{
		Directors: templateData.Directors,
		TextProps: templateConfig.Texto,
	}

	galleryComponent := &GalleryComponent{
		MediaIds: templateData.GalleryMediaIds,
	}

	// Build standard template composition
	return NewDiviTemplateComposer().
		AddComponent(&HeaderComponent{
			Title:           escapeHtml(templateData.Title),
			Subhead:         subhead,
			HeaderProps:     templateConfig.Header,
			BackgroundImage: templateData.BackgroundImage,
		}).
		AddComponent(&MenuComponent{
			MenuProps: templateConfig.Menu,
		}).
		AddComponent(&MainContentComponent{
			CreditsComponent:      creditsComponent,
			ContentNotesComponent: contentNotesComponent,
			Synopsis:              templateData.Synopsis,
			DirectorComponent:     directorComponent,
			GalleryComponent:      galleryComponent,
			SectionProps:          templateConfig.Contenido,
			TextProps:             templateConfig.Texto,
		}).
		AddComponent(&FooterComponent{
			ButtonText: buttonText,
			FooterProps: templateConfig.Footer,
		})
}

type DiviImageData struct {
	Encoded string `json:"encoded"`
	URL     string `json:"url"`
	ID      int    `json:"id"`
}

type DiviTemplateFile struct {
	Context      string                   `json:"context"`
	Data         map[string]string        `json:"data"`
	Presets      map[string]any           `json:"presets"`
	GlobalColors [][]any                  `json:"global_colors"`
	Images       map[string]DiviImageData `json:"images"`
	Thumbnails   []any                    `json:"thumbnails"`
}

// SaveDiviTemplateToFile writes <filmDir>/divi_template.json (Divi
// portability format) for already generated shortcodes. Its "images" list
// holds every cached media item; with EmbedImages each image is downloaded
// through downloader and embedded as base64.
func (s *DiviTemplateService) SaveDiviTemplateToFile(ctx context.Context, shortcodes string, cache *MediaCache, downloader MediaClient, filmDir string, wordpressPostID int) error {
	// Use WordPress Post ID instead of film title for better consistency
	projectID := fmt.Sprintf("%d", wordpressPostID)

	images := s.imagesData(ctx, cache, downloader)

	templateFile := &DiviTemplateFile{
		Context: "et_builder",
		Data: map[string]string{
			projectID: shortcodes,
		},
		Presets: map[string]any{
			"et_pb_row": map[string]any{
				"presets": map[string]any{
					"_initial": map[string]any{
						"name":    "Fila Preset 1",
						"version": BuilderVersion,
						"settings": map[string]any{
							"use_custom_gutter": "off",
							"gutter_width":      "1",
							"width":             "90%",
							"module_alignment":  "center",
						},
					},
				},
				"default": "_initial",
			},
		},
		GlobalColors: [][]any{
			{"gcid-primary-color", map[string]any{"color": ColorDark, "active": "yes"}},
			{"gcid-secondary-color", map[string]any{"color": ColorPrimary, "active": "yes"}},
			{"gcid-heading-color", map[string]any{"color": ColorSecondary, "active": "yes"}},
			{"gcid-body-color", map[string]any{"color": ColorBody, "active": "yes"}},
		},
		Images:     images,
		Thumbnails: []any{},
	}

	templatePath := filepath.Join(filmDir, "divi_template.json")
	templateJSON, err := json.MarshalIndent(templateFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template file: %v", err)
	}

	if err := os.WriteFile(templatePath, templateJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write template file: %v", err)
	}

	logger.Get().Infof("save_divi_template", "Saved Divi template to: %s", templatePath)

	return nil
}

// imagesData builds the "images" entry of the template file, keyed by URL.
func (s *DiviTemplateService) imagesData(ctx context.Context, cache *MediaCache, downloader MediaClient) map[string]DiviImageData {
	images := make(map[string]DiviImageData)
	for _, id := range cache.IDs() {
		media, _ := cache.Get(id)
		encoded := ""
		if s.EmbedImages && downloader != nil {
			data, err := downloader.Download(ctx, media.SourceURL)
			if err != nil {
				logger.Get().Warnf("fetch_wordpress_images", "Failed to encode image %d: %v", id, err)
			} else {
				encoded = base64.StdEncoding.EncodeToString(data)
			}
		}
		images[media.SourceURL] = DiviImageData{
			Encoded: encoded,
			URL:     media.SourceURL,
			ID:      id,
		}
	}
	return images
}

// findDirectorImage returns the URL of the image whose title, file name or
// alt text contains the director's name; failing that, one whose title or
// file name contains a part (longer than 2 characters) of the name.
func (s *DiviTemplateService) findDirectorImage(directorName string, imageIds []int, cache *MediaCache) string {
	if len(imageIds) == 0 {
		return ""
	}

	cleanName := strings.ToLower(strings.TrimSpace(directorName))

	for _, imageID := range imageIds {
		media, ok := cache.Get(imageID)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(media.Title.String()), cleanName) ||
			strings.Contains(MediaFilenameStem(media.SourceURL), cleanName) ||
			strings.Contains(strings.ToLower(media.AltText), cleanName) {
			debug.Printf("Found director image for '%s': %s", directorName, media.SourceURL)
			return media.SourceURL
		}
	}

	nameParts := strings.Fields(cleanName)
	if len(nameParts) > 1 {
		for _, imageID := range imageIds {
			media, ok := cache.Get(imageID)
			if !ok {
				continue
			}
			title := strings.ToLower(media.Title.String())
			filename := MediaFilenameStem(media.SourceURL)
			for _, part := range nameParts {
				if len(part) > 2 && (strings.Contains(title, part) || strings.Contains(filename, part)) {
					debug.Printf("Found director image for '%s' (partial match with '%s'): %s", directorName, part, media.SourceURL)
					return media.SourceURL
				}
			}
		}
	}

	logger.Get().Warnf("find_director_image", "No director image found for '%s'", directorName)
	return ""
}

// Template component interface for composition
type TemplateComponent interface {
	Render() string
}

// Credits section component
type CreditsComponent struct {
	Directors []DirectorInfo
	Credits   Credits
}

func (c *CreditsComponent) Render() string {
	var creditsHTML strings.Builder

	if len(c.Directors) > 0 {
		var directorNames []string
		for _, director := range c.Directors {
			directorNames = append(directorNames, director.Name)
		}
		creditsHTML.WriteString(fmt.Sprintf(`<p><strong>Dirección:</strong> %s</p>`, escapeHtml(strings.Join(directorNames, ", "))))
	}

	// If OtherCredits is present, use it instead of individual credit fields
	if c.Credits.OtherCredits != "" {
		// Split by dots and join with <br> tags
		parts := strings.Split(c.Credits.OtherCredits, ".")
		var escapedParts []string
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				escapedParts = append(escapedParts, escapeHtml(trimmed))
			}
		}
		if len(escapedParts) > 0 {
			creditsHTML.WriteString(fmt.Sprintf(`<p>%s</p>`, strings.Join(escapedParts, "<br>")))
		}
	} else {
		// Fall back to individual credit fields if OtherCredits is not present
		if c.Credits.Production != "" {
			creditsHTML.WriteString(fmt.Sprintf(`<p><strong>Producción:</strong> %s</p>`, escapeHtml(c.Credits.Production)))
		}
		if c.Credits.Script != "" {
			creditsHTML.WriteString(fmt.Sprintf(`<p><strong>Guión:</strong> %s</p>`, escapeHtml(c.Credits.Script)))
		}
		if c.Credits.Photography != "" {
			creditsHTML.WriteString(fmt.Sprintf(`<p><strong>Cámara - Foto:</strong> %s</p>`, escapeHtml(c.Credits.Photography)))
		}
		if c.Credits.ArtDesign != "" {
			creditsHTML.WriteString(fmt.Sprintf(`<p><strong>Arte - Diseño:</strong> %s</p>`, escapeHtml(c.Credits.ArtDesign)))
		}
		if c.Credits.SoundMusic != "" {
			creditsHTML.WriteString(fmt.Sprintf(`<p><strong>Sonido - Música:</strong> %s</p>`, escapeHtml(c.Credits.SoundMusic)))
		}
		if c.Credits.Editing != "" {
			creditsHTML.WriteString(fmt.Sprintf(`<p><strong>Edición:</strong> %s</p>`, escapeHtml(c.Credits.Editing)))
		}
		if c.Credits.Cast != "" {
			creditsHTML.WriteString(fmt.Sprintf(`<p><strong>Intérpretes (especificar pronombres para subtítulos):</strong> %s</p>`, escapeHtml(c.Credits.Cast)))
		}
	}

	return creditsHTML.String()
}

// Director section component
type DirectorComponent struct {
	Directors []DirectorInfo
	TextProps Text
}

func (d *DirectorComponent) Render() string {
	var sections strings.Builder

	for _, director := range d.Directors {
		escapedName := escapeHtml(director.Name)
		escapedBio := escapeHtml(director.Bio)

		directorImage := director.ImageURL
		if directorImage == "" {
			directorImage = ""
		}

		sections.WriteString(fmt.Sprintf(`[et_pb_row column_structure="1_2,1_2" _builder_version="%s" %s %s][et_pb_column type="1_2" _builder_version="%s" %s %s][et_pb_image src="%s" alt="%s" title_text="%s" _builder_version="%s" %s %s][/et_pb_image][/et_pb_column][et_pb_column type="1_2" _builder_version="%s" %s %s][et_pb_text _builder_version="%s" text_font_size="15px" link_font="%s" link_text_color="%s" header_4_font="%s" header_4_text_color="%s" header_4_font_size="19px" background_color="%s" max_height_tablet="" max_height_phone="" max_height_last_edited="on|desktop" custom_padding="%s" %s box_shadow_color="%s" %s]<h4><span>%s</span></h4>
<p><span data-sheets-root="1">%s</span></p>[/et_pb_text][/et_pb_column][/et_pb_row]`,
			BuilderVersion, ModulePresetDefault, GlobalColorsInfo, BuilderVersion, ModulePresetDefault, GlobalColorsInfo,
			directorImage,
			escapedName,
			escapedName,
			BuilderVersion, ModulePresetDefault, GlobalColorsInfo, BuilderVersion, ModulePresetDefault, GlobalColorsInfo, BuilderVersion, FontBold, ColorCoral, FontBoldCaps, d.TextProps.Header4TextColor, ColorWhite, PaddingDirector, BoxShadowPreset3, d.TextProps.BoxShadowColor, GlobalColorsInfo,
			escapedName,
			escapedBio,
		))
	}

	return sections.String()
}

// Content notes component
type ContentNotesComponent struct {
	ContentNotes string
	NdcProps     Ndc
}

func (c *ContentNotesComponent) Render() string {
	if c.ContentNotes == "" {
		return ""
	}

	escapedNdc := escapeHtml(c.ContentNotes)
	return fmt.Sprintf(`
	[et_pb_text disabled_on="%s" _builder_version="%s" %s text_font="%s" text_text_color="%s" background_color="%s" custom_margin="%s" custom_padding="%s" %s box_shadow_color="%s" locked="off" %s]
		<p>
			<strong>NdC: <span data-sheets-root="1">%s</span><br />
			</strong>
		</p>
	[/et_pb_text]`,
		c.NdcProps.Text.DisabledOn, BuilderVersion, ModulePresetDefault, FontBold, c.NdcProps.Text.Color, c.NdcProps.Text.BackgroundColor, MarginStandard, PaddingNotes, BoxShadowPreset3, c.NdcProps.Text.BoxShadowColor, GlobalColorsInfo,
		escapedNdc,
	)
}

// Gallery component
type GalleryComponent struct {
	MediaIds string
}

func (g *GalleryComponent) Render() string {
	return fmt.Sprintf(`
	[et_pb_row _builder_version="%s" %s]
		[et_pb_column type="4_4" _builder_version="%s" %s]
			[et_pb_gallery gallery_ids="%s" fullwidth="on" _builder_version="%s" %s %s]
			[/et_pb_gallery]
		[/et_pb_column]
	[/et_pb_row]`,
		BuilderVersion, GlobalColorsInfo, BuilderVersion, GlobalColorsInfo, g.MediaIds, BuilderVersion, ModulePresetDefault, GlobalColorsInfo,
	)
}

// Header component
type HeaderComponent struct {
	Title           string
	Subhead         string
	BackgroundImage string
	HeaderProps     Header
}

func (h *HeaderComponent) Render() string {
	return fmt.Sprintf(`
	[et_pb_section fb_built="1" fullwidth="on" _builder_version="%s" %s]
		[et_pb_fullwidth_header title="%s" subhead="%s" _builder_version="%s" title_font="%s" title_text_color="%s" subhead_text_color="%s"  background_enable_color="off" use_background_color_gradient="on" background_color_gradient_stops="%s 0%%|#82d0d9 50%%|%s 100%%" background_image="%s" background_blend="multiply" width="99.9%%" custom_padding="20%%||2%%||false|false" custom_padding_tablet="" custom_padding_phone="" custom_padding_last_edited="on|desktop" %s]
		[/et_pb_fullwidth_header]
	[/et_pb_section]`,
		BuilderVersion, GlobalColorsInfo, h.Title, h.Subhead, BuilderVersion, FontBoldCaps, h.HeaderProps.TitleTextColor, h.HeaderProps.SubHeadTextColor, ColorPrimary, ColorSecondary, h.BackgroundImage, GlobalColorsInfo,
	)
}

// Menu component
type MenuComponent struct {
	MenuProps Menu
}

func (m *MenuComponent) Render() string {
	return fmt.Sprintf(`[et_pb_section fb_built="1" fullwidth="on" _builder_version="%s" %s %s][et_pb_fullwidth_menu menu_id="%s" active_link_color="%s" dropdown_menu_text_color="#ffcccc" mobile_menu_text_color="#ffcccc" cart_icon_color="#ffcccc" search_icon_color="#ffcccc" menu_icon_color="#ffcccc" _builder_version="%s" menu_font="Montserrat|700||on|||||" menu_text_color="%s" menu_font_size="12px" background_color="%s" background_image="%s" background_blend="overlay" text_orientation="right" menu_text_color_tablet="%s" menu_text_color_phone="%s" menu_text_color_last_edited="on|desktop" %s menu_text_color__hover_enabled="on|desktop" menu_text_color__hover="%s"][/et_pb_fullwidth_menu][/et_pb_section]`,
		BuilderVersion, ModulePresetDefault, GlobalColorsInfo, m.MenuProps.MenuId, m.MenuProps.ActiveLinkColor, BuilderVersion, m.MenuProps.MenuTextColor, m.MenuProps.BackgroundColor, m.MenuProps.BackgroundImage, ColorSecondary, ColorSecondary, GlobalColorsInfo, ColorLightGreen,
	)
}

// Main content section component
type MainContentComponent struct {
	SectionProps          Section
	TextProps             Text
	CreditsComponent      *CreditsComponent
	ContentNotesComponent *ContentNotesComponent
	Synopsis              string
	DirectorComponent     *DirectorComponent
	GalleryComponent      *GalleryComponent
}

type RowComponent struct {
	ColumnStruct   string
	BuilderVersion string
}

func (m *MainContentComponent) Render() string {
	escapedSinopsis := escapeHtml(m.Synopsis)
	creditsSection := m.CreditsComponent.Render()
	contentNotesSection := m.ContentNotesComponent.Render()
	directorSection := m.DirectorComponent.Render()
	galleryComponent := m.GalleryComponent.Render()

	return fmt.Sprintf(`
	[et_pb_section fb_built="1" _builder_version="%s" background_color="%s" use_background_color_gradient="on" background_color_gradient_stops="%s" background_color_gradient_start="%s" background_color_gradient_end="%s"]
		[et_pb_row column_structure="1_2,1_2" _builder_version="%s" %s]	
			[et_pb_column type="1_2" _builder_version="%s" %s]
				[et_pb_text _builder_version="%s" text_font_size="15px" header_4_font="%s" header_4_text_color="%s" header_4_font_size="19px" background_color="%s" max_height_tablet="" max_height_phone="" max_height_last_edited="on|desktop" custom_padding="%s" %s box_shadow_color="%s" %s]
					<h4><strong>FICHA TÉCNICA:</strong></h4>
					%s
				[/et_pb_text]
			[/et_pb_column]
			[et_pb_column type="1_2" _builder_version="%s" %s]
				[et_pb_text _builder_version="%s" text_font_size="15px" header_4_font="%s" header_4_text_color="%s" header_4_font_size="19px" background_color="%s" custom_padding="%s" %s box_shadow_color="%s" %s]
					<h4><strong>SINOPSIS:</strong></h4>
					<p class="p1">
						<span data-sheets-root="1">%s</span>
					</p>
				[/et_pb_text]
				%s
			[/et_pb_column]
		[/et_pb_row]
		%s
		%s
	[/et_pb_section]`,
		BuilderVersion, m.SectionProps.Background, m.SectionProps.BackgroundColorGradientStops, m.SectionProps.BackgroundColorGradientStart, m.SectionProps.BackgroundColorGradientEnd, BuilderVersion, GlobalColorsInfo, BuilderVersion, GlobalColorsInfo, BuilderVersion, FontBoldCaps, m.TextProps.Header4TextColor, ColorWhite, PaddingStandard, BoxShadowPreset3, m.TextProps.BoxShadowColor, GlobalColorsInfo,
		creditsSection,
		BuilderVersion, GlobalColorsInfo, BuilderVersion, FontBoldCaps, m.TextProps.Header4TextColor, ColorWhite, PaddingStandard, BoxShadowPreset3, m.TextProps.BoxShadowColor, GlobalColorsInfo,
		escapedSinopsis, contentNotesSection, directorSection, galleryComponent,
	)
}

// Footer component
type FooterComponent struct {
	ButtonText string
	FooterProps Footer
}

func (f *FooterComponent) Render() string {
	return fmt.Sprintf(`
	[et_pb_section fb_built="1" admin_label="Section" _builder_version="%s" background_image="%s" background_position="%s" min_height="294.8px" custom_margin="||||false|false" custom_padding="||||false|false" global_module="%s" saved_tabs="all" %s]
		[et_pb_row disabled_on="off|off|off" _builder_version="4.23.2" %s min_height="164.4px" %s]
			[et_pb_column type="4_4" _builder_version="4.17.4" %s %s]
				[et_pb_button button_url="@ET-DC@eyJkeW5hbWljIjp0cnVlLCJjb250ZW50IjoicG9zdF9saW5rX3VybF9wYWdlIiwic2V0dGluZ3MiOnsicG9zdF9pZCI6IjEwNDI0In19@" button_text="%s" button_alignment="center" disabled_on="on|on|on" module_class="popmake-6500" _builder_version="%s" _dynamic_attributes="button_url" %s %s button_text_color="%s" button_bg_color="%s" button_border_color="%s" button_font="%s" button_icon_color="%s" %s box_shadow_color="%s" disabled="on" %s button_text_color__hover_enabled="on|desktop" button_text_color__hover="%s" button_bg_color__hover_enabled="on|hover" button_bg_color__hover="%s" button_bg_enable_color__hover="on" button_border_color__hover_enabled="on|hover" button_border_color__hover="%s"]
				[/et_pb_button]
			[/et_pb_column]
		[/et_pb_row]
		[et_pb_row column_structure="1_3,1_3,1_3" _builder_version="%s" %s]\
			[et_pb_column type="1_3" _builder_version="%s" %s]
				[et_pb_social_media_follow icon_color="%s" icon_color_tablet="%s" icon_color_phone="%s" icon_color_last_edited="on|tablet" _builder_version="%s" background_color="RGBA(255,255,255,0)" %s button_text_color="%s" button_bg_color="%s" button_border_color="%s" text_orientation="center" custom_margin="||||false|false" %s]
					[et_pb_social_media_follow_network social_network="facebook" url="%s" icon_color="%s" _builder_version="%s" background_color="%s" %s %s]
						facebook
					[/et_pb_social_media_follow_network]
					[et_pb_social_media_follow_network social_network="instagram" url="%s" icon_color="%s" _builder_version="%s" background_color="%s" %s %s]
						instagram
					[/et_pb_social_media_follow_network]
					[et_pb_social_media_follow_network social_network="twitter" url="%s" icon_color="%s" _builder_version="%s" background_color="%s" %s %s]
						twitter
					[/et_pb_social_media_follow_network]
				[/et_pb_social_media_follow]
			[/et_pb_column]
			[et_pb_column type="1_3" _builder_version="%s" %s]
				[et_pb_text _builder_version="%s" text_text_color="%s" link_font="%s" link_text_color="%s" header_text_color="%s" text_orientation="center" text_text_align="center" %s link_text_color__hover_enabled="on|desktop"]
					<p><span style="color: %s;"><strong><a href="mailto:%s" target="_blank" rel="noopener noreferrer" style="color: %s;">%s</a></strong></span></p>
				[/et_pb_text]
			[/et_pb_column]
			[et_pb_column type="1_3" _builder_version="%s" %s]
				[et_pb_search button_color="%s" placeholder_color="%s" _builder_version="%s" form_field_background_color="RGBA(255,255,255,0)" form_field_text_color="%s" form_field_focus_background_color="RGBA(255,255,255,0)" form_field_focus_text_color="%s" button_font="%s" button_text_color="%s" button_font_size="12px" form_field_font_size="12px" background_color="rgba(0,0,0,0)" background_last_edited="on|phone" border_width_all="3px" border_color_all="%s" %s background__hover_enabled="on|desktop"]
				[/et_pb_search]
			[/et_pb_column]
		[/et_pb_row]
	[/et_pb_section]`,
		BuilderVersion, f.FooterProps.Section.BackgroundImage, f.FooterProps.Section.BackgroundPosition, f.FooterProps.Section.GlobalModule, GlobalColorsInfo, ModulePresetDefault, GlobalColorsInfo, ModulePresetDefault, GlobalColorsInfo, f.ButtonText, BuilderVersion, ModulePresetDefault, CustomButtonOn, f.FooterProps.Button.ButtonTextColor, f.FooterProps.Button.ButtonBgColor, f.FooterProps.Button.ButtonBorderColor, FontBold, f.FooterProps.Button.ButtonIconColor, BoxShadowPreset3, f.FooterProps.Button.BoxShadowColor, GlobalColorsInfo, ColorYellow, ColorCoral, ColorCoral,
		BuilderVersion, GlobalColorsInfo, BuilderVersion, GlobalColorsInfo, ColorPrimary, ColorPink, ColorPink, BuilderVersion, CustomButtonOn, ColorPrimary, ColorSecondary, ColorSecondary, GlobalColorsInfo, URLFacebook, ColorPrimary, BuilderVersion, ColorSecondary, BackgroundColorOn, GlobalColorsInfo, URLInstagram, ColorPrimary, BuilderVersion, ColorSecondary, BackgroundColorOn, GlobalColorsInfo, URLTwitter, ColorPrimary, BuilderVersion, ColorSecondary, BackgroundColorOn, GlobalColorsInfo,
		BuilderVersion, GlobalColorsInfo, BuilderVersion, ColorYellow, FontBold, ColorDark, ColorDark, GlobalColorsInfo, ColorDark, EmailContact, ColorDark, EmailContact,
		BuilderVersion, GlobalColorsInfo, ColorPrimary, ColorPrimary, BuilderVersion, ColorDark, ColorDark, FontExtraBold, ColorSecondary, ColorPrimary, GlobalColorsInfo,
	)
}

// Template composer - coordinates all components
type DiviTemplateComposer struct {
	components []TemplateComponent
}

func NewDiviTemplateComposer() *DiviTemplateComposer {
	return &DiviTemplateComposer{
		components: make([]TemplateComponent, 0),
	}
}

func (d *DiviTemplateComposer) AddComponent(component TemplateComponent) *DiviTemplateComposer {
	d.components = append(d.components, component)
	return d
}

func (d *DiviTemplateComposer) Compose() string {
	var result strings.Builder
	for _, component := range d.components {
		result.WriteString(component.Render())
	}
	return result.String()
}
