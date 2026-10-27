# 06 — Tests, docs, CI

Execution: plan Steps 0 and 8.

## 6.1 Tests

- Golden Divi output: `internal/services/testdata/golden_2026.txt`.
- Template files: `TestTemplateFiles_Validate`.
- Pure functions: `CreateWordPressSlug`, `parseDuration`,
  `ExtractFileIDFromURL`, `SanitizeFilename`, `isAllowedFolder`,
  `filterStillsImages`, `CreateMetadata`, `parseDate`, `matchCategory`,
  `cleanCategories`, `MediaInfo` round-trip, `sheet.Directors`.
- HTTP: `FetchMedia` chunking/pagination, credential redaction, media request
  budget (all with `httptest`).
- Orchestration: `internal/film` with in-memory fakes, no network.

## 6.2 CI

`.github/workflows/go.yml`:

- `build` job: `go build`, `go vet`, `go test -race -cover ./...`.
- `lint` job: `golangci-lint` (errcheck, staticcheck, govet, unused, gosec)
  with `.golangci.yml`. Introduced non-blocking in Step 0, blocking from
  Step 8.

## 6.3 Release

`.goreleaser.yaml`: linux/darwin/windows × amd64/arm64; archives include
`LICENSE`, `README.md`, `configuration.json.example`, `templates/**`,
`metadata/**`; `-X main.version={{.Version}}`, which is also passed to the
logger.

## 6.4 README outline (nine sections)

1. **Overview** — what the tool does end to end (sheet → Drive → images →
   WordPress media → project post → Divi template).
2. **Requirements** — Go version, Google service account with Sheets and
   Drive APIs, WordPress with the Divi `project` post type and an
   application password, Turso database.
3. **Installation** — release archives or `go build`.
4. **Configuration** — every `configuration.json` key, defaults, required
   fields.
5. **Data conventions** — required sheet columns, edition filter
   (`edition_prefix` + year), Drive folder layout (`Background`,
   `Featured Image`, `Stills`, `Dir`), director naming, multi-director rows.
6. **Year templates and metadata** — `templates/<year>.json` schema with
   required fields, `metadata/<year>.json` (cities/dates for the caption).
7. **Usage** — every flag (must equal `-h` output), common invocations,
   interactive vs non-interactive.
8. **Output** — `films/<title>/` layout, `divi_template.json`,
   `metadata.json`, logs, Turso metadata types (`wordpress`, `drive_files`,
   `wp_images`, `wordpress_media`, `director_media`).
9. **Development** — tests, lint, release, project layout, troubleshooting.
