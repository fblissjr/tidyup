# Changelog

## 0.2.0

### Added
- `--dry-run` flag to preview deletions without acting
- `--version` flag with build-time version injection
- `--json` flag for machine-readable output
- `--verbose` flag for scan progress on stderr
- `--exclude` flag for comma-separated path patterns to skip
- `--min-size` flag to filter by minimum venv size
- `--sort` flag to sort by size, age, or path
- `--trash` flag to move to ~/.Trash instead of permanent delete (macOS)
- `--log` flag to write timestamped deletion audit log
- Multiple positional args support (scan multiple directories)
- Exit codes for scripting: 0=nothing found, 1=found, 2=error

### Fixed
- Zero-time ghost detection: venvs with no readable markers now fall back to directory mtime instead of being flagged as infinitely old
- Swallowed errors from `os.UserHomeDir` and `filepath.Abs` now reported properly
- Removed unnecessary `atomic` usage in sequential deletion loop

### Changed
- Moved main logic into `run()` function returning exit codes
- Deletion counter changed from `atomic.Int32` to plain `int`

## 0.1.0

- Initial release
- Concurrent venv scanning with goroutines
- Activity detection via pyvenv.cfg and activation script mtimes
- Interactive safe-delete with y/N confirmation
- `--system` flag for uv cache locations
- Configurable age threshold and scan depth
