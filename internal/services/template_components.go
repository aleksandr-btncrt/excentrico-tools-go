package services

import (
	"embed"
	"html"
	"strings"
	"text/template"
)

// Divi page components. Each component's markup lives in
// templates/<name>.tmpl (embedded below) and is rendered with text/template
// using named fields. Values are not escaped automatically: templates call
// `esc` (html.EscapeString) on sheet text, exactly where the previous
// fmt.Sprintf rendering escaped it. The golden tests in divi_golden_test.go
// pin the output byte for byte.

//go:embed templates/*.tmpl
var componentTemplateFS embed.FS

var componentTemplates = template.Must(
	template.New("components").
		Funcs(template.FuncMap{
			"esc":  html.EscapeString,
			"join": strings.Join,
		}).
		ParseFS(componentTemplateFS, "templates/*.tmpl"),
)

// DiviConsts exposes the shared Divi constants to the templates, under the
// same names as the Go constants ({{.BuilderVersion}}, {{.ColorPrimary}}...).
type DiviConsts struct {
	BuilderVersion      string
	ModulePresetDefault string
	GlobalColorsInfo    string
	BoxShadowPreset3    string
	CustomButtonOn      string
	BackgroundColorOn   string
	FontBold            string
	FontBoldCaps        string
	FontExtraBold       string
	PaddingStandard     string
	PaddingDirector     string
	MarginStandard      string
	PaddingNotes        string
	ColorPrimary        string
	ColorSecondary      string
	ColorDark           string
	ColorYellow         string
	ColorCoral          string
	ColorLightGreen     string
	ColorWhite          string
	ColorPink           string
	URLFacebook         string
	URLInstagram        string
	URLTwitter          string
	EmailContact        string
}

var diviConsts = DiviConsts{
	BuilderVersion:      BuilderVersion,
	ModulePresetDefault: ModulePresetDefault,
	GlobalColorsInfo:    GlobalColorsInfo,
	BoxShadowPreset3:    BoxShadowPreset3,
	CustomButtonOn:      CustomButtonOn,
	BackgroundColorOn:   BackgroundColorOn,
	FontBold:            FontBold,
	FontBoldCaps:        FontBoldCaps,
	FontExtraBold:       FontExtraBold,
	PaddingStandard:     PaddingStandard,
	PaddingDirector:     PaddingDirector,
	MarginStandard:      MarginStandard,
	PaddingNotes:        PaddingNotes,
	ColorPrimary:        ColorPrimary,
	ColorSecondary:      ColorSecondary,
	ColorDark:           ColorDark,
	ColorYellow:         ColorYellow,
	ColorCoral:          ColorCoral,
	ColorLightGreen:     ColorLightGreen,
	ColorWhite:          ColorWhite,
	ColorPink:           ColorPink,
	URLFacebook:         URLFacebook,
	URLInstagram:        URLInstagram,
	URLTwitter:          URLTwitter,
	EmailContact:        EmailContact,
}

// render executes the named component template. The templates are parsed
// at init and their inputs are fixed structs, so an error here is a
// programming error (caught by the golden tests) and panics.
func render(name string, data any) string {
	var b strings.Builder
	if err := componentTemplates.ExecuteTemplate(&b, name, data); err != nil {
		panic("divi template " + name + ": " + err.Error())
	}
	return b.String()
}

// TemplateComponent is one block of the Divi page.
type TemplateComponent interface {
	Render() string
}

// CreditsComponent renders the FICHA TÉCNICA body (directors and credits).
type CreditsComponent struct {
	Directors []DirectorInfo
	Credits   Credits
}

func (c *CreditsComponent) Render() string {
	names := make([]string, 0, len(c.Directors))
	for _, d := range c.Directors {
		names = append(names, d.Name)
	}
	// "Otros créditos" is shown one sentence per line.
	var parts []string
	for _, p := range strings.Split(c.Credits.OtherCredits, ".") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return render("credits", struct {
		*CreditsComponent
		DirectorNames     []string
		OtherCreditsParts []string
	}{c, names, parts})
}

// DirectorComponent renders one image + bio row per director.
type DirectorComponent struct {
	Directors []DirectorInfo
	TextProps Text
}

func (d *DirectorComponent) Render() string {
	return render("director", struct {
		DiviConsts
		*DirectorComponent
	}{diviConsts, d})
}

// ContentNotesComponent renders the "NdC" box; empty when there are none.
type ContentNotesComponent struct {
	ContentNotes string
	NdcProps     Ndc
}

func (c *ContentNotesComponent) Render() string {
	return render("content_notes", struct {
		DiviConsts
		*ContentNotesComponent
	}{diviConsts, c})
}

// GalleryComponent renders the stills gallery.
type GalleryComponent struct {
	MediaIds string
}

func (g *GalleryComponent) Render() string {
	return render("gallery", struct {
		DiviConsts
		*GalleryComponent
	}{diviConsts, g})
}

// HeaderComponent renders the full-width header. Title must already be
// HTML-escaped.
type HeaderComponent struct {
	Title           string
	Subhead         string
	BackgroundImage string
	HeaderProps     Header
}

func (h *HeaderComponent) Render() string {
	return render("header", struct {
		DiviConsts
		*HeaderComponent
	}{diviConsts, h})
}

// MenuComponent renders the full-width navigation menu.
type MenuComponent struct {
	MenuProps Menu
}

func (m *MenuComponent) Render() string {
	return render("menu", struct {
		DiviConsts
		*MenuComponent
	}{diviConsts, m})
}

// MainContentComponent renders the main section: credits, synopsis,
// content notes, directors and gallery.
type MainContentComponent struct {
	SectionProps          Section
	TextProps             Text
	CreditsComponent      *CreditsComponent
	ContentNotesComponent *ContentNotesComponent
	Synopsis              string
	DirectorComponent     *DirectorComponent
	GalleryComponent      *GalleryComponent
}

func (m *MainContentComponent) Render() string {
	return render("main_content", struct {
		DiviConsts
		*MainContentComponent
		CreditsHTML      string
		ContentNotesHTML string
		DirectorsHTML    string
		GalleryHTML      string
	}{
		DiviConsts:           diviConsts,
		MainContentComponent: m,
		CreditsHTML:          m.CreditsComponent.Render(),
		ContentNotesHTML:     m.ContentNotesComponent.Render(),
		DirectorsHTML:        m.DirectorComponent.Render(),
		GalleryHTML:          m.GalleryComponent.Render(),
	})
}

// FooterComponent renders the footer section (button, social links,
// contact e-mail and search).
type FooterComponent struct {
	ButtonText  string
	FooterProps Footer
}

func (f *FooterComponent) Render() string {
	return render("footer", struct {
		DiviConsts
		*FooterComponent
	}{diviConsts, f})
}

// DiviTemplateComposer concatenates components in order.
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
