// Package sheet holds the Google Sheet schema: column headers, the Film row
// type and helpers that read it. It is the only place that knows the sheet's
// column names.
package sheet

// Column headers of the films sheet, exactly as they appear in the header
// row (including the double space in ColSinopsisCompacta).
const (
	ColTituloOriginal      = "TÍTULO ORIGINAL"
	ColDireccion           = "DIRECCIÓN"
	ColPais                = "PAIS"
	ColAno                 = "AÑO"
	ColDuracion            = "DURAC."
	ColEdicion             = "EDICIÓN"
	ColSeccion             = "SECCIÓN"
	ColTipo                = "TIPO"
	ColSocialEtiquetas     = "SOCIAL/ETIQUETAS"
	ColIdiomas             = "Idioma(s) / Language(s)"
	ColRelacionAspectRatio = "Relación / Aspect Ratio (4:3, 16:9 u otro)"
	ColSinopsisExtendida   = "Sinopsis extendida (máximo 70 palabras)"
	ColExtendedSynopsis    = "Extended synopsis (english)"
	ColSinopsisCompacta    = "Sinopsis compacta  (máximo 10 palabras)"
	ColShortSynopsis       = "Short Synopsis (log line - Uso Pink Label)"
	ColNotasContenido      = "Notas de contenido / Content notes (*)"
	ColNotaIntencion       = "Nota de intención"
	ColProduccion          = "Producción / Producer(s)"
	ColGuion               = "Guión"
	ColCamaraFoto          = "Cámara - Foto / Camera - Photography"
	ColArteDiseno          = "Arte - Diseño / Art/Design"
	ColSonidoMusica        = "Sonido - Música / Sound - Music"
	ColEdicionCredits      = "Edición / Editor(s)"
	ColInterpretes         = "Intérpretes (especificar pronombres para subtítulos)/ Cast (please specify pronouns for subtitles)"
	ColOtrosCreditos       = "Otros créditos / Other credits"
	ColFestivalesPremios   = "Festivales y premios / Festivals & Awards"
	ColBioRealizadorxs     = "Bio Realizadorxs / Filmaker's Bio (min 150 - max 1500 caracteres)"
	ColCorreoElectronico   = "Correo electrónico / Email"
	ColTelefono            = "Teléfono / Phone number"
	ColEnlaces             = "ENLACES"
	ColWebExcentrico       = "Web Excentrico"
	ColImagenesBaja        = "imágenes en baja"
	ColObsSubtitulos       = "Obs. Subtitulos"
	ColPublishedStatus     = "Published Status"
	ColCategoria           = "Categoría"
	ColMultiDir            = "Multi Dir"

	// ColName is a fallback title column used by some older sheets; it is
	// not a Film field and ends up in AdditionalFields.
	ColName = "Name"
)
