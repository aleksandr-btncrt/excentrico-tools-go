package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// TemplateData is the per-edition styling loaded from templates/<year>.json.
type TemplateData struct {
	Header    Header  `json:"header"`
	Menu      Menu    `json:"menu"`
	Contenido Section `json:"contenido"`
	Texto     Text    `json:"texto"`
	Ndc       Ndc     `json:"ndc"`
	Footer    Footer  `json:"footer"`
}

type Header struct {
	TitleTextColor        string `json:"title_text_color"`
	SubHeadTextColor      string `json:"subhead_text_color"`
	BackgroundEnableColor string `json:"background_enable_color"`
}

type Menu struct {
	MenuId          string `json:"menu_id,omitempty"`
	ActiveLinkColor string `json:"active_link_color,omitempty"`
	MenuTextColor   string `json:"menu_text_color,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`
	BackgroundImage string `json:"background_image,omitempty"`
}

type Section struct {
	Background                   string `json:"background_color,omitempty"`
	BackgroundColorGradientStops string `json:"background_color_gradient_stops,omitempty"`
	BackgroundColorGradientStart string `json:"background_color_gradient_start,omitempty"`
	BackgroundColorGradientEnd   string `json:"background_color_gradient_end,omitempty"`
}

type Text struct {
	Header4TextColor string `json:"header_4_text_color,omitempty"`
	BoxShadowColor   string `json:"box_shadow_color,omitempty"`
}

type Ndc struct {
	Text struct {
		DisabledOn      string `json:"disabled_on"`
		Color           string `json:"color"`
		BackgroundColor string `json:"background_color"`
		BoxShadowColor  string `json:"box_shadow_color"`
	} `json:"text"`
}

type Footer struct {
	Section struct {
		BackgroundImage    string `json:"background_image"`
		BackgroundPosition string `json:"background_position"`
		GlobalModule       string `json:"global_module"`
	} `json:"section"`
	Button struct {
		BoxShadowColor    string `json:"box_shadow_color"`
		ButtonIconColor   string `json:"button_icon_color"`
		ButtonBorderColor string `json:"button_border_color"`
		ButtonTextColor   string `json:"button_text_color"`
		ButtonBgColor     string `json:"button_bg_color"`
	} `json:"button"`
}

// Validate reports every required field that is empty. A template that
// passes Validate renders a structurally complete Divi page.
func (t TemplateData) Validate() error {
	required := []struct {
		path  string
		value string
	}{
		{"menu.menu_id", t.Menu.MenuId},
		{"header.title_text_color", t.Header.TitleTextColor},
		{"header.subhead_text_color", t.Header.SubHeadTextColor},
		{"contenido.background_color_gradient_stops", t.Contenido.BackgroundColorGradientStops},
		{"texto.header_4_text_color", t.Texto.Header4TextColor},
		{"ndc.text.color", t.Ndc.Text.Color},
		{"footer.section.global_module", t.Footer.Section.GlobalModule},
	}

	var missing []string
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			missing = append(missing, r.path)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("template is missing required fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// DecodeTemplateData strictly decodes a template (unknown keys are an error)
// and validates it.
func DecodeTemplateData(r io.Reader) (TemplateData, error) {
	var t TemplateData
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return TemplateData{}, fmt.Errorf("invalid template JSON: %w", err)
	}
	if err := t.Validate(); err != nil {
		return TemplateData{}, err
	}
	return t, nil
}

// LoadTemplateData reads, strictly decodes and validates a template file.
func LoadTemplateData(path string) (TemplateData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return TemplateData{}, fmt.Errorf("template file not found: %s", path)
		}
		return TemplateData{}, fmt.Errorf("failed to read template file %s: %w", path, err)
	}
	t, err := DecodeTemplateData(bytes.NewReader(data))
	if err != nil {
		return TemplateData{}, fmt.Errorf("%s: %w", path, err)
	}
	return t, nil
}
