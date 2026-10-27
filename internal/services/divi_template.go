package services

import (
	"context"
	"fmt"
	"html"
	"strings"

	"excentrico-tools-go/internal/sheet"
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
			ButtonText:  buttonText,
			FooterProps: templateConfig.Footer,
		})
}
