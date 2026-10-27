# 03 — Performance

Execution: plan Step 6.

## 3.1 N+1 media lookups

For every film, `GET /wp/v2/media/<id>` was issued once per image by each of
`findDirectorImage` (twice per director: exact then partial pass),
`selectBackgroundImageURL` (twice), `selectFeaturedMediaID`, and
`fetchWordPressImagesData` — and the whole template was generated twice. A
film with 3 directors and 20 images issued 200+ requests.

Fix: `WordPressService.FetchMedia(ctx, ids)` returns a `MediaCache` using
`GET /wp/v2/media?include=<ids>&per_page=100`, chunked by 100 IDs and
following `X-WP-TotalPages`. If the batch endpoint errors, it falls back to
per-ID `GetMedia`. The cache is built once in
`CreateOrUpdateWordPressProject` and passed to every selector.

Target: ≤ 5 `GET /wp/v2/media*` per film (1 batch + director searches for
uncached directors). Verified by `TestCreateOrUpdate_MediaRequestBudget`
(`internal/wordpress`), which drives the real `WordPressService` against an
`httptest` server with 3 directors / 20 images: the post step now issues
**1** media request (one `?include=` batch). `TestFetchMedia_*`
(`internal/services`) cover chunking by 100, `X-WP-TotalPages` pagination,
the per-ID fallback and cancellation. A live count from a run log is still
pending (see 3.5).

## 3.2 Single template generation

`CreateOrUpdateWordPressProject` generated the template, discarded the result,
and `SaveDiviTemplateToFile` generated it again. The template is now
generated once, after the post is created/updated, and
`SaveDiviTemplateToFile(ctx, shortcodes, cache, downloader, filmDir, postID)`
only writes it. The `images` list in the file is built from the media cache.

## 3.3 Filename stem helper

The "URL → lower-case filename without extension" logic was copied three
times; it is now `services.MediaFilenameStem()`, and the keyword match used
by the featured and background selectors is `services.MediaMatchesAny()`.

## 3.4 Embedded images

`divi_template.json` embedded every image as base64, downloading every image
on every run. This is now controlled by `-embed-images`
(`DiviTemplateService.EmbedImages`). With `-embed-images=false` the file has
`"encoded": ""` and nothing is downloaded; the `url` and `id` are still
present.

The default is still **on** (the existing behaviour): the plan only allows
turning it off after a real Divi import without embedded images has been
checked, and that has not happened yet.

### Divi import validation

Status: **pending manual validation.** Import one `divi_template.json`
generated with `-embed-images=false` into Divi (Portability → Import) on the
staging site and confirm that images resolve from their `url`. Record the
result here (date, Divi version, outcome); if it works, change the flag
default to `false` in `main.go`.

## 3.5 Live verification

Status: **pending.** Run one film with `-film "<title>" -debug` and count
`/wp/v2/media` requests in the run log (target ≤ 5). Blocked in this
environment because the configured service account gets HTTP 403 reading the
sheet (same on `main`).
