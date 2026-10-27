package sheet

import (
	"regexp"
	"strings"
)

// directorSplitRe matches the separators used between director names in the
// DIRECCIÓN column: ",", " + ", " y " and "&". Only whole-word "y" is a
// separator, so names containing the letter (e.g. "Maya") are kept intact.
var directorSplitRe = regexp.MustCompile(`\s*,\s*|\s+\+\s+|\s+y\s+|\s*&\s*`)

// SplitDirectors splits a director cell ("A, B y C", "A & B", "A + B") into
// trimmed, non-empty names.
func SplitDirectors(s string) []string {
	var names []string
	for _, p := range directorSplitRe.Split(s, -1) {
		if p = strings.TrimSpace(p); p != "" {
			names = append(names, p)
		}
	}
	return names
}

// Directors returns the film's director names. The DIRECCIÓN cell is split
// only when "Multi Dir" is "SI"; otherwise the whole cell is one name.
// It returns nil when DIRECCIÓN is empty.
func (f *Film) Directors() []string {
	d := strings.TrimSpace(f.Direccion)
	if d == "" {
		return nil
	}
	if f.IsMultiDir() {
		return SplitDirectors(d)
	}
	return []string{d}
}
