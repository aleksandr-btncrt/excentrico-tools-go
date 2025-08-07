package services

type FilmData struct {
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

	AdditionalFields map[string]string `json:"additional_fields,omitempty"`
}
