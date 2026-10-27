package sheet

import "strings"

// Options describes where the films live in the spreadsheet.
type Options struct {
	// Tab is the sheet tab name, e.g. "TODO".
	Tab string
	// EditionPrefix is prepended to a year to build the EDICIÓN value,
	// e.g. "Excéntrico " + "2026".
	EditionPrefix string
}

// DefaultOptions returns the layout of the festival's sheet.
func DefaultOptions() Options {
	return Options{Tab: "TODO", EditionPrefix: "Excéntrico "}
}

// DataRange is the A1 range of the whole film table (header row included).
func (o Options) DataRange() string { return o.Tab + "!A:ZZ" }

// HeaderRange is the A1 range of the header row.
func (o Options) HeaderRange() string { return o.Tab + "!1:1" }

// Edition returns the EDICIÓN value for year, e.g. "Excéntrico 2026".
func (o Options) Edition(year string) string { return o.EditionPrefix + year }

// Select keeps the films of the given edition year (all when year is empty)
// and, when title is set, the film whose TÍTULO ORIGINAL equals it. Both
// comparisons are case-insensitive and ignore surrounding spaces.
func (o Options) Select(films []*Film, year, title string) []*Film {
	edition := strings.TrimSpace(o.Edition(year))
	title = strings.TrimSpace(title)
	var out []*Film
	for _, f := range films {
		if year != "" && !strings.EqualFold(strings.TrimSpace(f.Edicion), edition) {
			continue
		}
		if title != "" && !strings.EqualFold(strings.TrimSpace(f.Title()), title) {
			continue
		}
		out = append(out, f)
	}
	return out
}
