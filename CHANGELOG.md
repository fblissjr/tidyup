# Changelog

## 0.4.0

### Added
- Multi-type scanning: node_modules, __pycache__, .pytest_cache, .mypy_cache, .ruff_cache, dist/, build/
- `-type` flag for comma-separated type selection
- `-all` flag to scan for all supported types
- Interactive selection: numbered list with range/individual picking when deleting
- Active venv protection: refuses to delete $VIRTUAL_ENV
- Path safety guards: refuses to delete system-critical paths
- Improved venv validation: requires bin/ or Scripts/ in addition to pyvenv.cfg
- Improved usage heuristics: checks site-packages mtimes for better staleness detection
- node_modules usage heuristic: checks .package-lock.json and parent lockfiles
- `safety.go` module with safety validation functions

### Changed
- VenvRecord renamed to Record with new Type field in JSON output
- Output text says "items" instead of "environments"
- `-system` flag warns when used without venv type
- Confirmation prompt replaced with interactive numbered selection
- `--confirm` skips interactive selection (automation backward compat)

### Fixed
- False positive: venvs created long ago but actively used no longer misidentified as stale
- False positive: directories with only pyvenv.cfg but no bin/ no longer treated as venvs

## 0.3.0

### Added
- `--confirm` flag to skip interactive y/N prompt for CI/automation
- Collision-safe `--trash`: appends timestamp suffix when basename already exists in ~/.Trash
- Graceful `--trash` fallback on non-macOS (warns and uses permanent delete)

### Changed
- Split main.go into scan.go, delete.go, output.go, and main.go for maintainability
- Extracted `options` struct to pass configuration cleanly between modules
- Makefile now builds from package (`.`) instead of single file

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
