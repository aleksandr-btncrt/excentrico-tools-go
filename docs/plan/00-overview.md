# Refactor & Hardening Plan — excentrico-tools-go

## Goal

Fix known correctness bugs, remove the credential leak, eliminate N+1 WordPress
calls, make the orchestration layer testable, and bring the README in line with
the actual application.

## Guiding decisions (confirmed)

- The Divi project taxonomy REST endpoint is `/wp/v2/project_category`.
  `GetCategories` (`/project-category`) and `SearchCategoriesWithParams`
  (`/categories`) are wrong and will be removed.
- `templates/2025.json` and `templates/2026.json` are both kept. Files in
  `templates/` come from Divi and are not edited by this refactor; a
  validator guarantees a template is structurally complete before processing
  starts (2025.json, an `et_pb_*`-keyed export, is reported as unsupported).
- PRs are stacked: each step's branch is based on the previous step's branch.
- No behavioural change to the Divi output for 2026 unless explicitly noted.

## Phases

| Phase | Doc | Scope | Risk |
|-------|-----|-------|------|
| 1 | 01-bugfixes.md | Correctness fixes, template validation | Low |
| 2 | 02-security-robustness.md | Credential logging, timeouts, sentinel errors, input | Low |
| 3 | 03-performance.md | Media cache, single template generation | Medium |
| 4 | 04-structure.md | Sheet schema package, interfaces, split divi_template.go | Medium-High |
| 5 | 05-logging-cli.md | Logger defaults, output streams, new flags | Low |
| 6 | 06-tests-docs-ci.md | Unit tests, lint, goreleaser config, README rewrite | Low |

The execution order and PR boundaries are in
`plans/2026-10-01-excentrico-refactor-course-of-action-v1.md`.

## Definition of done (whole plan)

- `go vet ./...` and `golangci-lint run` are clean.
- Processing a year with no `templates/<year>.json` fails fast with a clear
  message instead of panicking.
- A film with 3 directors and 20 images issues <= 5 `GET /wp/v2/media*`
  requests (previously 200+).
- `grep -r "authHeader" internal/ | grep -i printf` returns nothing.
- README matches `./excentrico-tools-go -h` output and the on-disk layout.
