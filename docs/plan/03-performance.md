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
`httptest` server with 3 directors / 20 images.

## 3.2 Single template generation

`CreateOrUpdateWordPressProject` generated the template, discarded the result,
and `SaveDiviTemplateToFile` generated it again. The template is now
generated once and `SaveDiviTemplateToFile(ctx, shortcodes, cache, filmDir,
postID)` only writes it.

## 3.3 Filename stem helper

The "URL → lower-case filename without extension" logic was copied three
times; it is now `mediaFilenameStem()`.

## 3.4 Embedded images

`divi_template.json` embedded every image as base64, downloading every image
on every run. This is now behind `-embed-images` (default **off**, producing
`"encoded": ""`; the `url` and `id` are still present).

### Divi import validation

Status: **pending manual validation.** Before changing the default, import one
`divi_template.json` generated without `-embed-images` into Divi
(Portability → Import) on the staging site and confirm that images resolve
from their `url`. Record the result here (date, Divi version, outcome). Until
then, use `-embed-images` for any template that must be imported on a site
where the media library does not already contain the images.
