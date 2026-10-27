# 01 — Correctness fixes

Execution: plan Steps 1 and 2.

## 1.1 Template validation and the nil-config panic

Problem: `loadYearTemplateConfig` returned `nil` on any problem (missing file,
bad JSON) and `main` only logged a warning. The nil `*TemplateData` was then
dereferenced inside `CreateStandardFilmTemplate`, panicking in the middle of a
run, after WordPress media had already been uploaded.

`templates/2025.json` used an older Divi-module-keyed schema
(`et_pb_fullwidth_header`, `et_pb_fullwidth_menu`, `et_pb_section`) which
silently decoded into an all-empty `TemplateData`.

Fix:

- Move the template types to `internal/services/template_data.go`.
- `Footer.Button` gets an explicit `json:"button"` tag and a new
  `ButtonBgColor` (`button_bg_color`) field, which 2026.json already carried
  but which was ignored. The footer button now uses it instead of the
  hard-coded `ColorSecondary` (intentional, documented output change).
- `TemplateData.Validate()` reports every missing required path.
- Templates are decoded strictly (`DisallowUnknownFields`) and validated;
  a missing or invalid template is fatal before any network work starts.
- `-template <path>` lets a template be used without `-year`.
- The template is passed by value everywhere.
- Files in `templates/` come from Divi and are **not edited** by this
  refactor. `templates/2025.json` (Divi module-keyed `et_pb_*` export) is
  left untouched; it does not match the `TemplateData` schema, so
  `-year 2025` now stops with an error naming the unknown key instead of
  rendering empty attributes.
- `TestTemplateFiles_Validate` keeps every supported file in `templates/`
  valid and asserts that the listed unsupported ones (2025.json) are rejected.

## 1.2 Category matching

`strings.ReplaceAll(name, "", "-")` inserted a dash between every rune, so the
first matching pass never matched. It is now `ReplaceAll(name, " ", "-")`.
`SearchCategories(year)` is hoisted out of the per-name loop, and matching is a
pure function `matchCategory`. The endpoints `/wp/v2/project-category` and
`/wp/v2/categories` were wrong and their callers (`GetCategories`,
`SearchCategoriesWithParams`) are deleted.

## 1.3 `wordpress_media` metadata

Stored as `[]map[string]any`; after a JSON round-trip `post_id` became a
`float64`, so `updateMediaMetadataWithPostID` (which asserted `int`) never
updated anything. A typed `models.MediaInfo` replaces the maps.

## 1.4 Unchecked type assertions on sheet cells

Nine `.(string)` assertions panicked on any non-string cell. They are replaced
with `utils.CellString` (and later by the `sheet` package's row decoder).

## 1.5 Caption / `metadata.json`

- The director string was transformed with `strings.Replace(d, "y", "&")`,
  which corrupted names ("Ray" → "RA&"). Directors are now split with the
  shared director regex and joined with `" & "`.
- `parseDate` ignored parse errors; it now uses `time.Parse` and returns an
  error. `CreateMetadata` returns `(string, error)`; a malformed date in
  `metadata/<year>.json` fails that film with a message naming the date.
- The caption was written to `films/<raw title>/metadata.json` before the
  directory existed (it is created under the *sanitised* title), and the
  write error was ignored. Writing moved into `Processor.ProcessSingleFilm`
  after `MkdirAll`, as JSON `{"caption": "..."}`, and errors propagate.

## 1.6 Drive listing pagination

`ListFiles` read only the first page (100 items). It now follows
`NextPageToken` with `PageSize(1000)` and requests `modifiedTime`.

## 1.7 WordPress auth and payload

- `Password` is used when `ApplicationPassword` is empty (config validation
  already allowed this).
- `Type: "post"` is no longer sent when creating a `project`.

## 1.8 Menu selection

The chosen WordPress menu was only used as a "template" string and then
discarded; the rendered `menu_id` came from the template file. The selected
menu's ID now overrides `menu.menu_id`. Not selecting a menu is no longer an
abort — the template's `menu_id` is used. `-nav-menu` is renamed `-menu-slug`
(old name kept as an alias).

Details decided during implementation:

- With `-menu-slug`, the slug must match a WordPress menu; otherwise the run
  stops before processing. Interactively, an empty answer keeps the
  template's `menu_id`.
- `GetNavMenus`' menu-locations fallback used to synthesise IDs (1, 2, ...).
  Those are not menu term IDs, so it now leaves `ID` at 0 and the template's
  `menu_id` is kept (with a notice) for such menus.
- A search-categories failure is logged and the post is saved without
  categories, as before, but now costs one request per film instead of one
  per category name.
