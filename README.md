# Excentrico Tools Go

Command-line tool that publishes the films of an Excéntrico festival edition
to the festival's WordPress (Divi) site.

## 1. Overview

For every film row of the selected edition in the festival Google Sheet, the
tool:

1. downloads the images from the film's Google Drive folder (`ENLACES`
   column; only the allowed subfolders, see [Data conventions](#5-data-conventions));
2. resizes each image to an optimised `*_web.jpg` copy;
3. uploads the copies to the WordPress media library, reusing media that was
   already uploaded (and director portraits shared between films);
4. creates the film's Divi `project` post as a **draft**, or updates it if it
   was created by an earlier run, with categories from `SECCIÓN` and a
   featured image;
5. renders the Divi page (shortcodes) from the film data and the edition's
   year template, stores it in the post and writes
   `films/<title>/divi_template.json` for Divi Portability imports;
6. writes the film caption to `films/<title>/metadata.json`.

Progress is stored in a Turso database, so the tool can be rerun safely: files
that were already downloaded or uploaded are skipped, and existing posts are
updated instead of duplicated.

## 2. Requirements

- **Go 1.25+**, only if you build from source (see `go.mod`).
- **Google Cloud service account** with the Google Sheets API and Google Drive
  API enabled. The festival sheet and the film Drive folders must be shared
  with the service account's e-mail address.
- **WordPress** with Divi (the `project` post type) and a user with an
  [application password](https://wordpress.org/documentation/article/application-passwords/).
- **Turso** (libSQL) database URL and auth token.

## 3. Installation

**Release archive (recommended).** Download the archive for your platform
from the GitHub releases page (linux/darwin/windows, amd64/arm64) and extract
it. It contains the binary, `LICENSE`, this README,
`configuration.json.example`, `templates/` and `metadata/`.

**From source:**

```bash
git clone https://github.com/aleksandr-btncrt/excentrico-tools-go.git
cd excentrico-tools-go
go build -o excentrico-tools-go .    # or ./build.sh
./excentrico-tools-go -version
```

## 4. Configuration

The tool reads `configuration.json` and the Google credentials file from the
current directory. To create a starting file (written with mode `0600`
because it holds secrets), run:

```bash
./excentrico-tools-go -create-config
```

Then edit it, using `configuration.json.example` as a reference:

| Key | Required | Default | Description |
|---|---|---|---|
| `google_credentials_path` | yes | `credentials.json` | Service account key file |
| `google_sheet_id` | yes | — | ID of the festival Google Sheet (from its URL) |
| `wordpress_config.base_url` | yes | — | Site URL, e.g. `https://excentricofest.com` |
| `wordpress_config.username` | yes | — | WordPress user |
| `wordpress_config.application_password` | yes¹ | — | Application password for that user |
| `wordpress_config.password` | yes¹ | — | Used only when `application_password` is empty |
| `image_config.max_width` / `max_height` | no | `1920` / `1080` | Bounds of the `*_web.jpg` copies |
| `image_config.quality` | no | `85` | JPEG quality of the copies |
| `turso_config.database_url` | yes | — | `libsql://…turso.io` |
| `turso_config.auth_token` | yes | — | Turso auth token |
| `sheet_tab` | no | `TODO` | Sheet tab holding the films |
| `edition_prefix` | no | `Excéntrico ` | `EDICIÓN` value = prefix + year (note the trailing space) |

¹ One of the two is required; prefer the application password.

To check the configuration without writing anything, run
`./excentrico-tools-go -check`. It tests WordPress authentication, the Turso
connection and that the sheet's header row can be read.

## 5. Data conventions

### Google Sheet

- The first row of the `sheet_tab` tab holds the column headers, which must
  match exactly. The column names live in `internal/sheet/columns.go`.
- A row is part of an edition when its `EDICIÓN` equals
  `edition_prefix + year`, e.g. `Excéntrico 2026` (case-insensitive).
- Columns the tool uses:

| Column | Used for |
|---|---|
| `TÍTULO ORIGINAL` | Post title, slug, film directory name, film identity |
| `DIRECCIÓN` | Directors (caption, Divi director blocks, portrait lookup) |
| `Multi Dir` | `SI` → `DIRECCIÓN` holds several names |
| `SECCIÓN` | Comma-separated WordPress categories, caption, slug |
| `PAIS`, `AÑO`, `DURAC.` | Film details (`DURAC.` as `h:mm:ss` or `mm:ss`) |
| `Sinopsis extendida (máximo 70 palabras)` | Synopsis block |
| `Notas de contenido / Content notes (*)` | Content notes block (hidden when empty) |
| `Producción / Producer(s)`, `Guión`, `Cámara - Foto / …`, `Arte - Diseño / …`, `Sonido - Música / …`, `Edición / Editor(s)`, `Intérpretes …`, `Otros créditos / Other credits` | Credits block |
| `Bio Realizadorxs / Filmaker's Bio (…)` | Director bio |
| `ENLACES` | Google Drive folder URL with the film's images |

- **Multiple directors:** when `Multi Dir` is `SI`, the director names are
  split on `,`, ` y `, ` & ` and ` + ` (whole words only, so a name like
  "Maya" stays intact). Every director gets a block, and the caption joins
  the names with ` & `.

### Google Drive folder (`ENLACES`)

Only images in these subfolders (any depth, case-insensitive) are used:

| Folder | Purpose |
|---|---|
| `Background` | Header background image (first match for background/header/fondo/bg) |
| `Featured Image` | Post featured image |
| `Stills` | Gallery images |
| `Dir` | Director portraits; the file name should contain the director's name |

Director portraits are cached across films by director name. A director who
appears in several films is uploaded only once.

### WordPress categories

Each `SECCIÓN` entry is matched against the categories returned by a search
for the edition year. The dashed form (`Competencia Nacional` →
`competencia-nacional`) is tried first, then the plain name, and the match is
case-insensitive.

## 6. Year templates and metadata

### `templates/<year>.json`

The colours, menu and footer module of the Divi page for an edition. The
tool picks it with `-year`, or you can name a file with `-template`.
Templates are decoded strictly: **unknown keys and missing required fields
stop the run** before anything is downloaded or uploaded.

```json
{
  "header":    { "title_text_color": "#…", "subhead_text_color": "#…", "background_enable_color": "off" },
  "menu":      { "menu_id": "163", "active_link_color": "#…", "menu_text_color": "#…",
                 "background_color": "RGBA(…)", "background_image": "https://…" },
  "contenido": { "background_color": "#…", "background_color_gradient_stops": "#… 0%|#… 100%",
                 "background_color_gradient_start": "#…", "background_color_gradient_end": "#…" },
  "texto":     { "header_4_text_color": "#…", "box_shadow_color": "#…" },
  "ndc":       { "text": { "disabled_on": "off|off|off", "color": "#…", "background_color": "#…", "box_shadow_color": "#…" } },
  "footer":    { "section": { "background_image": "https://…", "background_position": "top_center", "global_module": "12678" },
                 "button":  { "box_shadow_color": "#…", "button_icon_color": "#…", "button_border_color": "#…",
                              "button_text_color": "#…", "button_bg_color": "#…" } }
}
```

Required fields: `menu.menu_id`, `header.title_text_color`,
`header.subhead_text_color`, `contenido.background_color_gradient_stops`,
`texto.header_4_text_color`, `ndc.text.color`,
`footer.section.global_module`. `templates/2026.json` is the reference.

> `templates/2025.json` is still a raw Divi export (`et_pb_*` keys) and is
> rejected until it is converted to the layout above.

The menu ID of a WordPress menu chosen with `-menu-slug` (or interactively)
overrides `menu.menu_id`.

### `metadata/<year>.json` (optional)

Used for the film caption:

```json
{ "cities": ["Valparaíso"], "dates": [["2026-01-23", "2026-01-30"]] }
```

Caption format, with dates in Spanish:
`SECCIÓN YEAR - Title - DIRECTOR & DIRECTOR - Programación Excéntrico YEAR City del viernes 23 de enero al viernes 30 de enero`.
Dates must be in `YYYY-MM-DD` format; an invalid date fails that film.

## 7. Usage

```text
  -check
    	Check WordPress auth, Turso connectivity and the sheet header row, then exit
  -create-config
    	Create a default configuration file
  -debug
    	Write debug events to the log file
  -dry-run
    	Read the sheet and report what would be processed; no downloads, uploads or writes
  -embed-images
    	Embed images as base64 in divi_template.json (use -embed-images=false for URL-only, much smaller files) (default true)
  -film string
    	Process only the film with this TÍTULO ORIGINAL (case-insensitive)
  -menu string
    	Action to run: configuration | process
  -menu-slug string
    	WordPress navigation menu slug; its ID overrides menu.menu_id from the template
  -nav-menu string
    	Deprecated alias for -menu-slug
  -non-interactive
    	Never prompt; use flags and defaults
  -template string
    	Path to a template JSON file (required when -year is not given)
  -version
    	Print the version and exit
  -year string
    	Edition year; selects templates/<year>.json and metadata/<year>.json
```

Common invocations:

```bash
# Check configuration and connectivity
./excentrico-tools-go -check

# See what would be processed for 2026 without changing anything
./excentrico-tools-go -non-interactive -year 2026 -dry-run

# Process the whole 2026 edition with a specific WordPress menu
./excentrico-tools-go -non-interactive -year 2026 -menu-slug menu-2026

# Reprocess a single film
./excentrico-tools-go -non-interactive -year 2026 -film "Título de la película"

# Interactive: choose the action, year and menu from prompts
./excentrico-tools-go
```

**Interactive vs non-interactive.** Without `-menu` the tool asks for the
action, and without `-year` it asks for the year. Without `-menu-slug` it
lists the WordPress menus to choose from. With `-non-interactive` it never
asks: the action defaults to `process` and the template's `menu_id` is used.
An empty answer, or piped stdin, counts as no answer.

**Output.** The terminal shows one line per film, e.g.
`[3/12] "Film" → post #1234, 18 images`, followed by a summary. If a film
fails, it is reported and the run moves on to the next film. Ctrl-C stops
the run before the next film, cancelling any requests in flight. Failures
that stop the run before processing starts (configuration, template) exit
with status 1.

## 8. Output

```text
films/<title>/
  Background/ Featured Image/ Stills/ Dir/   downloaded originals + *_web.jpg copies
  divi_template.json                          Divi Portability import of the page
  metadata.json                               {"caption": "…"}
logs/run-<timestamp>-<nanos>.log              one JSON event per line, per run
```

- `divi_template.json` holds the page shortcodes and an `images` map. With
  `-embed-images` (the default) each image is embedded as base64. With
  `-embed-images=false` only URLs and IDs are written, which gives a much
  smaller file.
- The log file records one structured event per operation (for example
  `process_single_film`, `create_wordpress_post` and `read_sheet`), with
  durations and errors. Credentials are never logged. Request bodies are
  logged only with `-debug`, and response bodies are cut off at 2 KB.
- **Turso metadata types**, stored per film unless noted otherwise:

| Type | Content |
|---|---|
| `wordpress` | Post ID, slug, status and link of the film's project post |
| `drive_files` | Drive files downloaded from the allowed folders (name, folder) |
| `wp_images` | Local file name → WordPress media ID |
| `wordpress_media` | Uploaded media (ID, URL, local path, post ID) |
| `director_media` | Global (`__directors__`): director name → media ID/URL |

## 9. Development

```bash
go build ./... && go vet ./...
go test -race -cover ./...        # fully offline; HTTP is faked with httptest
golangci-lint run ./...           # blocking in CI (.golangci.yml)
```

- **Golden output:** `internal/services/testdata/golden_2026*.txt` pins the
  generated Divi shortcodes. Regenerate them only for an intentional change,
  with `go test ./internal/services -run TestGolden -update`, and review the
  diff.
- **Release:** pushing a tag runs GoReleaser (`.goreleaser.yaml`), which
  sets the version with `-X main.version=<tag>`. To try it locally, run
  `goreleaser release --snapshot --clean`.
- **CI:** `.github/workflows/go.yml` runs build, vet, race tests and lint on
  pushes to `main` and on PRs to `main` and `refactor/**`.

### Project layout

```text
main.go                    flags, prompts, template/metadata loading, run()
internal/app               App wiring, sheet read, film loop, -check
internal/film              per-film pipeline + consumer interfaces (deps.go)
internal/sheet             sheet schema: columns, Film, directors, edition filter
internal/drive             Drive folder listing/download/resize
internal/wordpress         media upload, post create/update
internal/services          API clients (WordPress, Drive, Sheets, Turso, images),
                           media cache, Divi rendering (templates/*.tmpl)
internal/config, logger, debug, models, utils
templates/, metadata/      per-year Divi template and caption data
docs/plan/, plans/         refactor plan
```

### Troubleshooting

- **`template file not found` / `invalid template JSON: unknown field`:**
  add or convert `templates/<year>.json` (see section 6).
- **`google_sheet_id is not configured`:** set it in `configuration.json`.
- **Sheet `403`/access denied:** share the sheet (and the Drive folders)
  with the service account e-mail from `credentials.json`.
- **WordPress `401`:** check `username` and `application_password`, and
  `base_url` (no trailing `/wp-json`).
- **No films matched:** compare `EDICIÓN` in the sheet with
  `edition_prefix + year`, and `-film` with `TÍTULO ORIGINAL`.
- **Director image not found:** put the portrait in the film's `Dir`
  folder, with the director's name in the file name.

## License

MIT, see [LICENSE](LICENSE).
