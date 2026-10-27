# Excentrico Tools Go — Course of Action

## Objective

Execute the six-phase refactor plan (bug fixes → security → performance → structure → logging/CLI → tests/docs/CI) as a sequence of small, independently verifiable PRs. Every step lists the files it touches and the check that proves it is done. Decisions already confirmed: the taxonomy endpoint is `/wp/v2/project_category`; both `templates/*.json` files are kept; files in `templates/` come from Divi and are not edited by this refactor (`2025.json` is left as-is and is reported as unsupported). PRs are stacked: each step's branch is based on the previous step's branch.

Ground rule for every step: `go build ./... && go vet ./... && go test ./...` must pass before the step is marked complete.

---

## Implementation Plan

### Step 0 — Bootstrap (one PR)

- [x] 0.1 Create `docs/plan/00-overview.md` … `06-tests-docs-ci.md` from the agreed plan text so the plan lives in the repo and PRs can link to section numbers.
- [x] 0.2 Capture a **baseline golden output**: run the current code against one fixture film (hand-written `sheet.Film`-equivalent map + fake image IDs) and check the generated shortcode string into `internal/services/testdata/golden_2026.txt`. This is the safety net for every later change that touches `divi_template.go`. (Needs only a tiny harness: call `GenerateDiviShortcodeTemplate` with a `TemplateData` loaded from `templates/2026.json`.)
- [x] 0.3 Add `golangci-lint` config (`.golangci.yml`: errcheck, staticcheck, govet, unused, gosec) and the lint job to `.github/workflows/go.yml`, initially **non-blocking** (`continue-on-error: true`) so the baseline finding count is visible. Flip to blocking at Step 6.

### Step 1 — Stop the crashes (PR: "fix: template validation and nil-config panic")

- [x] 1.1 Create `internal/services/template_data.go`; move `TemplateData`, `Header`, `Menu`, `Section`, `Text`, `Ndc`, `Footer` out of `divi_template.go:128-195`. Add `json:"button"` tag to `Footer.Button` and new field `ButtonBgColor string \`json:"button_bg_color"\``.
- [x] 1.2 Add `func (t TemplateData) Validate() error` listing every missing required path (`menu.menu_id`, `header.title_text_color`, `header.subhead_text_color`, `contenido.background_color_gradient_stops`, `texto.header_4_text_color`, `ndc.text.color`, `footer.section.global_module`).
- [x] 1.3 In `main.go` `loadYearTemplateConfig` (`main.go:308-344`): decode with `DisallowUnknownFields()`, call `Validate()`, and return an error instead of nil. In `main` treat a missing/invalid template as fatal when a year is given; add `-template <path>` flag for the no-year case. Remove the "warn and continue" branch at `main.go:132-138`.
- [x] 1.4 Change `GenerateCompleteTemplate`, `CreateStandardFilmTemplate`, `SaveDiviTemplateToFile`, `CreateOrUpdateWordPressProject`, `ProcessSingleFilm`, `ProcessFilms` to take `TemplateData` by value.
- [x] 1.5 Use `FooterProps.Button.ButtonBgColor` in `FooterComponent.Render` in place of the hard-coded `ColorSecondary` (`divi_template.go:941`, 2nd `button_bg_color` arg).
- [!] 1.6 **Not done — by owner decision (2026-10-27): files in `templates/` come from Divi and are not to be edited by this refactor. `2025.json` stays as-is and is listed as unsupported in the template test; loading it fails fast with a readable error.** Original item: Rewrite `templates/2025.json` into the current schema: `header.*` from `et_pb_fullwidth_header` (prefix `#` on both colours), `menu.menu_id` = `"137"`, `contenido.*` from `et_pb_section`, and fill the remaining `menu`/`texto`/`ndc`/`footer` values from the constants at `divi_template.go:18-39` plus `URLHeaderBackground` for images.
- [x] 1.7 Add `TestTemplateFiles_Validate` that globs `../../templates/*.json`, decodes strictly, and asserts `Validate()==nil` for each. (Adjusted: files listed as unsupported — currently `2025.json` — must instead fail to load.)
- [x] 1.8 Verify: `./excentrico-tools-go -menu process -year 1999` exits 1 with a readable message; golden test from 0.2 still passes for 2026.

### Step 2 — Remaining correctness fixes (PR: "fix: category match, media post_id, caption, metadata.json, pagination")

- [x] 2.1 `wordpress.go:744`: `ReplaceAll(..., " ", "-")`; hoist `SearchCategories(year)` above the loop; extract `matchCategory(cats, name) *WordPressCategory`; delete `GetCategories` and `SearchCategoriesWithParams`.
- [x] 2.2 Add `models.MediaInfo` struct; replace every `[]map[string]any` for `wordpress_media` (`wordpress/operations.go:160,257,294,539`; `divi_template.go:294-320`). This fixes the `float64`/`int` bug in `updateMediaMetadataWithPostID`.
- [x] 2.3 Add `utils.CellString(any) string`; replace all nine `.(string)` assertions on sheet data.
- [x] 2.4 `app.go:276-312` `CreateMetadata`: split directors with the shared regex (temporarily copy from `wordpress/operations.go:68`; de-duplicated in Step 5), join with `" & "`, upper-case. `parseDate` → `time.Parse` returning `(string, error)`.
- [x] 2.5 Move film caption writing into `Processor.ProcessSingleFilm` after `MkdirAll`; write `{"caption": …}` JSON to `filmDir/metadata.json`; return the error. Delete `App.SaveMetadata`.
- [x] 2.6 `google_drive.go:94-106`: paginate with `PageToken`, `PageSize(1000)`, add `modifiedTime` to `Fields`.
- [x] 2.7 `wordpress.go:116`: fall back to `Password` when `ApplicationPassword` is empty. Drop `Type: "post"` from the post payload (`wordpress/operations.go:413`).
- [x] 2.8 Menu selection: after the user picks a WP menu in `main.go:143-202`, set `templateConfig.Menu.MenuId = strconv.Itoa(menu.ID)`; remove the abort at `main.go:215-219`; rename `-nav-menu` → `-menu-slug` keeping the old name as alias.
- [x] 2.9 Unit tests: `CreateMetadata`, `parseDate`, `matchCategory`, `cleanCategories`, `MediaInfo` round-trip, `CellString`.

### Step 3 — Security and robustness (PR: "chore: remove credential logging, add timeouts, sentinel errors")

- [ ] 3.1 Delete auth-header output at `wordpress.go:122` and `:687`. Add a test asserting the base64 auth string never reaches debug output.
- [ ] 3.2 `makeRequest` (`wordpress.go:668-672`): log payload size always, payload body only when `debug.IsEnabled()`; truncate `http_response_body` to 2 KB.
- [ ] 3.3 `http.Client{Timeout: 120s}`; replace `http.Get` in `downloadAndEncodeImage` with a `WordPressService.Download` method on the same client.
- [ ] 3.4 Thread `context.Context` from `main` (with `signal.NotifyContext`) through `App.ProcessFilms` → `ProcessSingleFilm` → drive/wordpress ops; use `NewRequestWithContext`.
- [ ] 3.5 `turso_service.go`: `ErrMetadataNotFound` sentinel wrapped with `%w`; replace the six `strings.Contains` checks with `errors.Is`.
- [ ] 3.6 `app.go:117`: return error instead of `log.Fatal`; move the `google_sheet_id` required check into `config.Load()`.
- [ ] 3.7 `main.go` prompts: `bufio.Scanner` helper; handle EOF/piped stdin.
- [ ] 3.8 Check errors from `writer.WriteField` (`wordpress.go:309,313,400,404`).

### Step 4 — Logging and CLI (PR: "feat: CLI flags and sane logger defaults")

- [ ] 4.1 `logger.go:94` default `sampleRate` 1.0; add `OperationTracker.Important()` bypassing sampling; mark `process_single_film`, `create_wordpress_post`, `update_wordpress_post`.
- [ ] 4.2 JSON events → log file only; new `logger.Progress()` to stderr; `[i/N] "<title>" → post #<id>, <n> images` lines in `processFilteredObjects`.
- [ ] 4.3 Remove the `op.Fail` + `log.Fatalf` duplicates and repeated `process_films` trackers in `main.go`; introduce `func run() error` with a single exit path.
- [ ] 4.4 Flags: `-non-interactive`, `-film "<title>"`, `-dry-run`, `-check` (wires existing `TestConnection` + Turso ping + sheet header read), `-template <path>` (from 1.3).
- [ ] 4.5 Replace stray `log.Printf`/`fmt.Printf` in services with tracker events.
- [ ] 4.6 Verify: run with `-non-interactive -year 2026 -dry-run`; terminal shows only progress lines; log file contains one `process_single_film` per film.

### Step 5 — Structure: schema package and interfaces (PR: "refactor: sheet package and consumer interfaces")

- [ ] 5.1 Create `internal/sheet/` with `columns.go` (all header constants from `ConvertObjToFilmData`), `film.go` (move `FilmData` → `sheet.Film`, delete `services/film_data_service.go`), `row.go` (`RowToFilm(headers, row)`), `directors.go` (`(*Film).Directors()` — single regex), `options.go` (`Tab`, `EditionPrefix`).
- [ ] 5.2 Add `sheet_tab` and `edition_prefix` to `config.Config` with defaults `"TODO"` / `"Excéntrico "`; update `configuration.json.example` and `CreateDefaultConfig`.
- [ ] 5.3 `App.ProcessFilms` produces `[]*sheet.Film`; `Processor`, `drive.ProcessGoogleDriveFiles`, `wordpress.UploadMediaToWordPress`, `CreateOrUpdateWordPressProject` take `*sheet.Film`. Delete `ConvertObjToFilmData`, `parseDirectorNames`, `parseDirectors`, `utils.CellString` (no longer needed). Port `TestParseDirectorNames` to `sheet.Directors()`.
- [ ] 5.4 Create `internal/film/deps.go` with `DriveClient`, `ImageResizer`, `WordPressClient`, `MetadataStore` interfaces; `NewProcessor` and the operation functions accept them; `app.New` wires concrete services. Convert `TursoService` typed helpers into free functions over `MetadataStore`.
- [ ] 5.5 Add `internal/film/fakes_test.go` and the orchestration tests: director de-dup across two films, update-existing-post path.
- [ ] 5.6 Verify: golden test unchanged; `go test ./internal/film/...` runs with no network.

### Step 6 — Performance (PR: "perf: media cache and single template generation")

- [ ] 6.1 `internal/services/media_cache.go`: `FetchMedia(ids) (*MediaCache, error)` using `?include=`, chunked by 100, following `X-WP-TotalPages`; add to `WordPressClient` interface.
- [ ] 6.2 Build the cache once in `CreateOrUpdateWordPressProject`; pass it into `findDirectorImage`, `selectBackgroundImageURL`, `selectFeaturedMediaID`, `fetchWordPressImagesData`.
- [ ] 6.3 Generate the template once; `SaveDiviTemplateToFile(shortcodes, cache, filmDir, postID)`; delete the discarded call at `wordpress/operations.go:392`.
- [ ] 6.4 `mediaFilenameStem()` helper replacing the three copies of URL→filename logic.
- [ ] 6.5 `-embed-images` flag (default off → `encoded: ""`). Validate against one real Divi import before changing the default; note the result in `docs/plan/03-performance.md`.
- [ ] 6.6 `httptest` test for `FetchMedia` chunking/pagination.
- [ ] 6.7 Verify: count `/wp/v2/media` requests in a run log — ≤ 5 per film.

### Step 7 — Structure: split and template-ise Divi rendering (PR: "refactor: split divi_template.go, text/template rendering")

- [ ] 7.1 Split into `template_components.go`, `template_media.go`, `template_file.go` (types already moved in 1.1). Delete `RowComponent` and the no-op `if directorImage == ""` branch.
- [ ] 7.2 Move each component's markup into `internal/services/templates/*.tmpl` with `//go:embed`; render via `text/template` with named fields.
- [ ] 7.3 Golden test from 0.2 must produce a byte-identical string (whitespace normalisation allowed only if documented).
- [ ] 7.4 Delete unused API: `UploadMedia`, `GetPost`, `GetPosts`, `DeletePost`, `GetTags`, `SearchTags`, `WriteRange`, `AppendRow`, `CreateSpreadsheet`, `GetSpreadsheet`. Extract `buildMultipart()` from `UploadMediaFromFile`.
- [ ] 7.5 `go mod tidy`; remove the commented Bubble Tea block from `go.mod`.

### Step 8 — Tests, CI, docs (PR: "docs: README rewrite; ci: lint blocking, goreleaser")

- [ ] 8.1 Remaining pure-function tests: `CreateWordPressSlug`, `parseDuration`, `ExtractFileIDFromURL`, `SanitizeFilename`, `isAllowedFolder`, `filterStillsImages`.
- [ ] 8.2 Flip lint job to blocking; add `go test -race -cover ./...`.
- [ ] 8.3 Add `.goreleaser.yaml` (linux/darwin/windows × amd64/arm64; archive `LICENSE`, `README.md`, `configuration.json.example`, `templates/**`, `metadata/**`; `-X main.version`), and set `logger.version` from it.
- [ ] 8.4 Rewrite `README.md` per the nine-section outline in `06-tests-docs-ci.md`; remove the non-existent API examples and `examples/` references; document sheet/Drive folder conventions, template schema, all flags, Turso metadata types.

---

## Verification Criteria

- Each PR passes `go build`, `go vet`, `go test`, and (from Step 8) `golangci-lint` in CI.
- Golden shortcode output for the 2026 fixture is identical from Step 0.2 through Step 7.
- `templates/2026.json` passes `Validate()`; `templates/2025.json` (unedited Divi export) is rejected with a readable error; a year without a template file exits non-zero with a human-readable message.
- No log line anywhere contains the WordPress auth header.
- A film with 3 directors / 20 images issues ≤ 5 `GET /wp/v2/media*` requests.
- `films/<title>/metadata.json` exists and is valid JSON after a run; director names in the caption retain their letters.
- `go test ./internal/film/...` completes with no network access.
- README flag list equals `./excentrico-tools-go -h` output.

## Potential Risks and Mitigations

1. **Divi output drifts during the `text/template` conversion**
   Mitigation: golden file captured *before* any rendering change (Step 0.2) and asserted in every subsequent step.
2. **2025 template cannot be processed after strict validation**
   Mitigation: by owner decision `templates/` is not edited; processing 2025 fails fast with an error naming the unknown key until the file is converted by the owner.
3. **`DisallowUnknownFields` rejects a template that worked before**
   Mitigation: that is the intent — it surfaces exactly the 2025-style mismatch; the error message lists the offending key.
4. **`?include=` media batching behaves differently on the live WP instance (plugins, auth on media)**
   Mitigation: `-check` flag exercises it; keep per-ID `GetMedia` as fallback when the batch endpoint errors.
5. **Interface extraction (Step 5.4) is a wide mechanical change**
   Mitigation: do 5.1–5.3 (schema) and 5.4 (interfaces) as two commits in the same PR; no behaviour change is permitted in either, and the orchestration tests in 5.5 are added immediately after.
6. **Changing `sampleRate` to 1.0 increases log volume**
   Mitigation: volume is tens of films × ~20 events — trivial; logs are per-run files already.

## Alternative Approaches

1. **Skip Step 7 (template-ising the Divi markup)** and only split the file: lower effort, but the 60-argument `Sprintf` calls remain the riskiest code to edit; recommended to keep Step 7 but it can be deferred indefinitely without blocking anything else.
2. **Do Step 6 before Step 5** (performance before interfaces): possible — `MediaCache` can be passed as a concrete type — but every signature touched in Step 6 would be touched again in Step 5. Ordering 5 → 6 touches each signature once.
3. **Replace the custom logger with `log/slog`** rather than tuning it: cleaner long-term, but the `OperationTracker` API is used in ~80 call sites; a swap is a larger diff than the three targeted fixes in Step 4. Revisit after Step 8 if desired.
