package sheet

import (
	"fmt"
	"strings"
)

// RowToFilm maps one data row onto a Film using the header row. Cells of
// any type are converted to strings (nil and missing trailing cells become
// ""). Columns with an empty header are ignored; non-empty cells of unknown
// columns go to AdditionalFields.
func RowToFilm(headers []string, row []any) *Film {
	f := &Film{AdditionalFields: make(map[string]string)}
	for j, header := range headers {
		if header == "" {
			continue
		}
		value := ""
		if j < len(row) {
			value = cellString(row[j])
		}
		if field := f.field(header); field != nil {
			*field = value
		} else if value != "" {
			f.AdditionalFields[header] = value
		}
	}
	return f
}

// Films converts a sheet (header row first) into Films. It returns nil when
// there are no data rows.
func Films(data [][]any) []*Film {
	if len(data) < 2 {
		return nil
	}
	headers := Headers(data[0])
	films := make([]*Film, 0, len(data)-1)
	for _, row := range data[1:] {
		films = append(films, RowToFilm(headers, row))
	}
	return films
}

// Headers converts the header row cells to strings.
func Headers(row []any) []string {
	headers := make([]string, len(row))
	for i, cell := range row {
		headers[i] = strings.TrimSpace(cellString(cell))
	}
	return headers
}

// cellString converts a Sheets/XLSX cell value to a string without panicking.
func cellString(v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case string:
		return s
	default:
		return fmt.Sprint(s)
	}
}

// field returns a pointer to the Film field for a column header, or nil.
func (f *Film) field(header string) *string {
	switch header {
	case ColTituloOriginal:
		return &f.TituloOriginal
	case ColDireccion:
		return &f.Direccion
	case ColPais:
		return &f.Pais
	case ColAno:
		return &f.Ano
	case ColDuracion:
		return &f.Duracion
	case ColEdicion:
		return &f.Edicion
	case ColSeccion:
		return &f.Seccion
	case ColTipo:
		return &f.Tipo
	case ColSocialEtiquetas:
		return &f.SocialEtiquetas
	case ColIdiomas:
		return &f.Idiomas
	case ColRelacionAspectRatio:
		return &f.RelacionAspectRatio
	case ColSinopsisExtendida:
		return &f.SinopsisExtendida
	case ColExtendedSynopsis:
		return &f.ExtendedSynopsis
	case ColSinopsisCompacta:
		return &f.SinopsisCompacta
	case ColShortSynopsis:
		return &f.ShortSynopsis
	case ColNotasContenido:
		return &f.NotasContenido
	case ColNotaIntencion:
		return &f.NotaIntencion
	case ColProduccion:
		return &f.Produccion
	case ColGuion:
		return &f.Guion
	case ColCamaraFoto:
		return &f.CamaraFoto
	case ColArteDiseno:
		return &f.ArteDiseno
	case ColSonidoMusica:
		return &f.SonidoMusica
	case ColEdicionCredits:
		return &f.EdicionCredits
	case ColInterpretes:
		return &f.Interpretes
	case ColOtrosCreditos:
		return &f.OtrosCreditos
	case ColFestivalesPremios:
		return &f.FestivalesPremios
	case ColBioRealizadorxs:
		return &f.BioRealizadorxs
	case ColCorreoElectronico:
		return &f.CorreoElectronico
	case ColTelefono:
		return &f.Telefono
	case ColEnlaces:
		return &f.Enlaces
	case ColWebExcentrico:
		return &f.WebExcentrico
	case ColImagenesBaja:
		return &f.ImagenesBaja
	case ColObsSubtitulos:
		return &f.ObsSubtitulos
	case ColPublishedStatus:
		return &f.PublishedStatus
	case ColCategoria:
		return &f.Categoria
	case ColMultiDir:
		return &f.MultiDir
	}
	return nil
}
