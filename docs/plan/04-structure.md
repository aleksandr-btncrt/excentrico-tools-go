# 04 — Structure

Execution: plan Steps 5 and 7.

## 4.1 `internal/sheet` — one place for the sheet schema

Column names were string literals repeated across `app`, `film`,
`wordpress` and `services`, rows travelled as `map[string]any`, and
directors were parsed by two different functions.

- `columns.go` — every header constant.
- `film.go` — `sheet.Film` (formerly `services.FilmData`).
- `row.go` — `RowToFilm(headers, row)`; cells of any type are converted to
  strings, unknown columns go to `AdditionalFields`.
- `directors.go` — `(*Film).Directors()` and `SplitDirectors` (one regex).
- `options.go` — `Options{Tab, EditionPrefix}`; configurable through
  `sheet_tab` / `edition_prefix` in `configuration.json` (defaults `"TODO"`
  and `"Excéntrico "`).

## 4.2 Consumer interfaces

`internal/film/deps.go` declares `DriveClient`, `ImageResizer`,
`WordPressClient` and `MetadataStore`. They embed the narrow interfaces the
`drive`, `wordpress` and `services` packages declare for their own
operations, so there is no import cycle and every operation can be driven by
fakes. `app.New` wires the concrete services. `TursoService`'s typed helpers
become free functions over `services.MetadataStore`.

`internal/film/fakes_test.go` provides in-memory fakes; orchestration tests
cover director de-duplication across films and the update-existing-post path,
with no network access.

## 4.3 Splitting `divi_template.go`

- `template_data.go` — template configuration types and validation.
- `template_components.go` — Divi components and composer.
- `template_media.go` — media selection (director / background / stills).
- `template_file.go` — `divi_template.json` writer.
- `templates/*.tmpl` — the component markup, embedded with `//go:embed` and
  rendered with `text/template` using named fields instead of 60-argument
  `Sprintf` calls.

The golden test (`testdata/golden_2026.txt`) guarantees byte-identical
output.

## 4.4 Dead code

Removed: `UploadMedia`, `GetPost`, `GetPosts`, `DeletePost`, `GetTags`,
`SearchTags`, `WriteRange`, `AppendRow`, `CreateSpreadsheet`,
`GetSpreadsheet`, `RowComponent`, and the commented-out Bubble Tea block in
`go.mod`. `buildMultipart()` is extracted from `UploadMediaFromFile`.
