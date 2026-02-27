# tidyup

Go CLI tool for finding and cleaning unused dev artifacts (venvs, node_modules, caches, build dirs).

## Project Structure

Single Go package (`package main`), all source files in root. No subdirectories.
- `main.go` -- CLI flags, entry point, type parsing
- `scan.go` -- filesystem walking, type detection, usage heuristics
- `safety.go` -- deletion safety checks (active venv, protected paths, venv validation)
- `delete.go` -- interactive selection, deletion logic, trash support
- `output.go` -- Record type, JSON/text output, sorting

## Build & Test

- `make build` -- builds binary with version injection
- `make test` -- runs `go test -v -count=1 ./...`
- `make install` -- builds + copies to /usr/local/bin (sudo)
- Test files: `*_test.go` colocated with source

## Conventions

- Go 1.21, no external dependencies
- Tab indentation (gofmt standard)
- Commit messages: short version-prefixed first line, detail in body
- `Record` is the core data type (formerly `VenvRecord`); always includes `Type` field
- `options` struct carries all CLI flags through the call chain
- Type detection: name-based (fast) for most types, content-based for venvs
- `dist/` and `build/` require parent directory validation (too generic alone)

## Gotchas

- Edit tool often fails on tab-indented Go files; use Write for full file rewrites when Edit string matching fails
- macOS: `cat -A` doesn't exist; use `od -c` or `hexdump` for whitespace debugging
