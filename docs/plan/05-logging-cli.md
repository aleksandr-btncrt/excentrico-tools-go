# 05 — Logging and CLI

Execution: plan Step 4.

## 5.1 Sampling

The logger sampled 10% of successful events by default, so most successful
films left no trace. Default `sampleRate` is now 1.0. `OperationTracker.Important()`
bypasses sampling for `process_single_film`, `create_wordpress_post` and
`update_wordpress_post` even if sampling is lowered.

## 5.2 Output streams

- JSON wide events go to the per-run log file only (`logs/run-*.log`); if the
  file cannot be opened they fall back to stderr.
- `logger.Progress()` writes human-readable lines to stderr:
  `[i/N] "<title>" → post #<id>, <n> images`.

## 5.3 Single exit path

`main` delegates to `run() error`; duplicate `op.Fail` + `log.Fatalf` pairs
and repeated `process_films` trackers are removed.

## 5.4 Flags

| Flag | Purpose |
|------|---------|
| `-menu` | `configuration` or `process` |
| `-year` | Edition year; selects `templates/<year>.json` and `metadata/<year>.json` |
| `-template` | Explicit template path (required when `-year` is omitted) |
| `-menu-slug` (`-nav-menu`) | WordPress menu slug whose ID overrides `menu.menu_id` |
| `-film` | Process only the film with this title |
| `-non-interactive` | Never prompt; use flags and defaults |
| `-dry-run` | Read the sheet and report what would be processed; no writes |
| `-check` | Check WordPress auth, Turso connectivity and sheet headers, then exit |
| `-embed-images` | Embed base64 images in `divi_template.json` |
| `-debug` | Verbose debug output |
| `-create-config` | Write a default `configuration.json` |
| `-version` | Print the version |

## 5.5 Stray prints

`log.Printf` / `fmt.Printf` calls inside services are replaced by tracker
events, so the terminal only shows progress.

## Details decided during implementation

- `-embed-images` lands with plan Step 6.5 and `-version` with Step 8.3; the
  table above is the end state.
- `debug.Printf` writes `"level":"debug"` events to the log file (not the
  terminal); `-debug` only controls whether they are recorded.
- Exactly one `process_single_film` event per film: the duplicate tracker in
  `app` and the wrapper trackers in `film.Processor` (which repeated the
  `process_drive_files` / `upload_wordpress_media` /
  `create_update_wordpress_project` events already emitted by the called
  functions) are removed. A caption error is logged as `build_caption`.
- `CreateOrUpdateWordPressProject` returns the post ID and
  `ProcessSingleFilm` returns `{PostID, Images}` for the progress line.
- Exit code: failed films are reported (`[i/N] "<title>" → FAILED: ...` and
  a final `Done:` line) but, as before, do not make the run exit non-zero.
- Sheet rows: an empty header cell no longer shifts the following columns
  (previously nil headers were dropped, misaligning later values).
- `-check` uses `TestConnection` (now returns the user name instead of
  printing), a new `TursoService.Ping`, and reads row 1 of the `TODO` tab.
- `-non-interactive` skips the menu, year and WordPress menu prompts; the
  configuration menu refuses to run with it.
