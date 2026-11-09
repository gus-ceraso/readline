# bobo's readline fork

This fork carries terminal fixes used by [bobo](https://github.com/gus-ceraso/bobo).
The `bobo` branch is based on upstream `reeflective/readline` v1.3.0, preserving
upstream history, license, module path, and tests. The upstream `master` branch
is retained separately.

Consumers keep importing `github.com/reeflective/readline` and use a Go module
`replace` directive pointing to a pinned revision of
`github.com/gus-ceraso/readline`. Retaining the original module path avoids
rewriting the library's internal imports and consumers' imports.

## Changes

- An empty `comment-begin` disables comment highlighting instead of matching
  every input line as a comment.
- The opt-in `preserve-cursor` setting suppresses cursor-style changes and the
  reset on return from `Readline`, leaving the terminal's cursor settings alone.
- The opt-in `cache-cursor-position` setting reuses the absolute input origin
  while editing. The renderer already accounts for its own scrolling. A new
  primary prompt or changed terminal dimensions invalidates the cache. Normal
  keystrokes therefore repaint in one buffered frame without a mid-frame DSR
  query. The default behavior for other consumers is unchanged.
- `Shell.BeforeRead` runs on the editor goroutine after each redraw, before
  waiting for keys. `Keys.RequestRefresh` wakes this callback safely, allowing
  signal handling and shutdown without concurrent editor access. Returning an
  error ends `Readline`.
- `Printf` and `PrintTransientf` invalidate the cached origin when they print a
  fresh prompt.
- `multiline-column-custom` is honored on every logical continuation line.
  Indicator placement accounts for wrapped rows, so a custom `> ` marker does
  not overwrite text on soft-wrapped lines.
- Cursor-position reads preserve typeahead before and after a cursor report,
  including reports split across reads.
- `Keys.QueueInput` appends raw bytes on the editor goroutine, so cooked terminal
  typeahead can be handed back to readline without changing byte order or
  converting partial UTF-8 to replacement characters.
- Resize watchers unregister their signal subscriptions when a read ends.

## Verification

Run `go test -race ./...` and `go vet ./...` in this repository. Regression tests
cover disabled comment highlighting, raw input queueing, cursor preservation,
position caching, custom multiline markers after wrapping, and the editor-thread
callback, alongside upstream's tests.

Bobo's own tests additionally exercise subprocesses, real Linux PTYs, and a
terminal emulator, including terminal resize, DSR/typeahead handling, and cooked
echo/buffered input while the model runs without an active readline prompt.
