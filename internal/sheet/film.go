package sheet

import "strings"

// Film is one row of the films sheet (formerly services.FilmData).
type Film struct {
	TituloOriginal      string `json:"titulo_original"`
	Direccion           string `json:"direccion"`
	Pais                string `json:"pais"`
	Ano                 string `json:"ano"`
	Duracion            string `json:"duracion"`
	Edicion             string `json:"edicion"`
	Seccion             string `json:"seccion"`
	Tipo                string `json:"tipo"`
	SocialEtiquetas     string `json:"social_etiquetas"`
	Idiomas             string `json:"idiomas"`
	RelacionAspectRatio string `json:"relacion_aspect_ratio"`
	SinopsisExtendida   string `json:"sinopsis_extendida"`
	ExtendedSynopsis    string `json:"extended_synopsis"`
	SinopsisCompacta    string `json:"sinopsis_compacta"`
	ShortSynopsis       string `json:"short_synopsis"`
	NotasContenido      string `json:"notas_contenido"`
	NotaIntencion       string `json:"nota_intencion"`
	Produccion          string `json:"produccion"`
	Guion               string `json:"guion"`
	CamaraFoto          string `json:"camara_foto"`
	ArteDiseno          string `json:"arte_diseno"`
	SonidoMusica        string `json:"sonido_musica"`
	EdicionCredits      string `json:"edicion_credits"`
	Interpretes         string `json:"interpretes"`
	OtrosCreditos       string `json:"otros_creditos"`
	FestivalesPremios   string `json:"festivales_premios"`
	BioRealizadorxs     string `json:"bio_realizadorxs"`
	CorreoElectronico   string `json:"correo_electronico"`
	Telefono            string `json:"telefono"`
	Enlaces             string `json:"enlaces"`
	WebExcentrico       string `json:"web_excentrico"`
	ImagenesBaja        string `json:"imagenes_baja"`
	ObsSubtitulos       string `json:"obs_subtitulos"`
	PublishedStatus     string `json:"published_status"`
	Categoria           string `json:"categoria"`
	MultiDir            string `json:"multi_dir"`

	// AdditionalFields holds non-empty cells of columns that are not Film
	// fields, keyed by header.
	AdditionalFields map[string]string `json:"additional_fields,omitempty"`
}

// UnnamedFilm is the title used when a row has no title.
const UnnamedFilm = "unnamed_film"

// Title returns TÍTULO ORIGINAL, falling back to the "Name" column and then
// to UnnamedFilm. It is the film's identity: the local directory and the
// metadata key are derived from it.
func (f *Film) Title() string {
	if t := strings.TrimSpace(f.TituloOriginal); t != "" {
		return f.TituloOriginal
	}
	if t := strings.TrimSpace(f.AdditionalFields[ColName]); t != "" {
		return f.AdditionalFields[ColName]
	}
	return UnnamedFilm
}

// HasTitle reports whether TÍTULO ORIGINAL is set.
func (f *Film) HasTitle() bool {
	return strings.TrimSpace(f.TituloOriginal) != ""
}

// IsMultiDir reports whether the "Multi Dir" column is "SI" (any case).
func (f *Film) IsMultiDir() bool {
	return strings.EqualFold(strings.TrimSpace(f.MultiDir), "SI")
}
