# 02 — Security and robustness

Execution: plan Step 3.

## 2.1 Credential logging

`NewWordPressService` and `makeRequest` printed the full `Authorization`
header (base64 of `user:password`) in debug mode. Both prints are removed and a
test asserts the encoded credentials never reach debug or event output.

## 2.2 Payload logging

`makeRequest` logged every JSON request body in full. Now:

- `http_request_payload_size` is always logged;
- `http_request_payload` only when `-debug` is on;
- `http_response_body` on errors is truncated to 2 KB.

## 2.3 Timeouts

The WordPress client had no timeout and image downloads used the default
`http.Get`. A single `http.Client{Timeout: 120s}` is used, and image
downloads go through `WordPressService.Download` on that client.

## 2.4 Cancellation

`main` creates a context with `signal.NotifyContext(os.Interrupt, SIGTERM)`.
It is threaded through `App.ProcessFilms` → `Processor.ProcessSingleFilm` →
Drive and WordPress operations; HTTP requests use `NewRequestWithContext`
and Google API calls use `.Context(ctx)`. Ctrl-C stops between requests
instead of killing the process mid-write.

## 2.5 Sentinel errors

`TursoService.GetMetadata` returns an error wrapping `ErrMetadataNotFound`;
callers use `errors.Is` instead of `strings.Contains(err.Error(), ...)`.

## 2.6 Fatal exits inside library code

`App.ProcessFilms` called `log.Fatal` when `google_sheet_id` was empty. The
check moves into `config.Load()` and the function returns errors.

## 2.7 Interactive prompts

`fmt.Scanln` read a single whitespace-separated token and mis-handled EOF
(piped stdin). Prompts use a `bufio.Scanner` helper that reads full lines and
treats EOF as "no answer".

## 2.8 Unchecked multipart errors

`writer.WriteField` / `writer.Close` errors are checked.

## Details decided during implementation

- `ProcessFilms` keeps a defensive empty-`google_sheet_id` check, but it
  returns an error instead of exiting.
- The film loop checks `ctx.Err()` before each film, so an interrupt ends the
  run with "Processing interrupted after N of M films" rather than starting
  the next film.
- `Download` does not send the WordPress `Authorization` header: image URLs
  may point at a CDN or another host.
- Google Sheets `ReadRange` and the Drive service methods take a context too
  (they are part of the same run).
