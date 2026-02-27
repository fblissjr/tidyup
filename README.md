# tidyup

> Last updated: 2026-02-27

Lightweight Go CLI to identify and clean up unused development artifacts: Python virtual environments, node_modules, caches, and build outputs. Optimized for macOS and `uv` users.

## Features

- **Multi-Type Scanning** -- Detects venvs, node_modules, __pycache__, .pytest_cache, .mypy_cache, .ruff_cache, dist/, and build/.
- **Advanced Activity Detection** -- Type-specific usage heuristics (activation scripts, lockfiles, site-packages, file mtimes) instead of unreliable directory access times.
- **Safety Hardening** -- Refuses to delete active venvs ($VIRTUAL_ENV), system-critical paths, and invalid venvs (pyvenv.cfg without bin/).
- **Interactive Selection** -- Numbered list with range/individual picking when deleting. No more all-or-nothing.
- **Concurrent Scanning** -- Uses goroutines to calculate directory sizes in parallel.
- **System-Wide Awareness** -- With `--system`, automatically includes standard uv cache locations.
- **Safe Deletion** -- Optional `--dry-run`, optional `--trash` (macOS) to move to Trash instead of permanent delete.
- **Machine-Readable Output** -- `--json` flag for scripting and piping to `jq`.
- **Auditable** -- `--log` writes a timestamped deletion log.

## Installation

```bash
git clone https://github.com/fblissjr/tidyup.git
cd tidyup
make install
```

This builds the binary and moves it to `/usr/local/bin` (requires sudo).

## Supported Types

| Type | Directory | Detection | Usage Heuristic |
|------|-----------|-----------|-----------------|
| `venv` | `pyvenv.cfg` + `bin/` or `Scripts/` | Content-based | Activation scripts, pyvenv.cfg, site-packages mtimes |
| `node_modules` | `node_modules/` | Name-based | .package-lock.json, parent lockfiles, dir mtime |
| `pycache` | `__pycache__/` | Name-based | Newest file mtime |
| `pytest_cache` | `.pytest_cache/` | Name-based | Newest file mtime |
| `mypy_cache` | `.mypy_cache/` | Name-based | Newest file mtime |
| `ruff_cache` | `.ruff_cache/` | Name-based | Newest file mtime |
| `dist` | `dist/` | Name + parent validation | Newest file mtime |
| `build` | `build/` | Name + parent validation | Newest file mtime |

`dist/` and `build/` require `pyproject.toml`, `setup.py`, `setup.cfg`, or `package.json` in the parent directory to avoid false positives.

## Usage

### Quick Start

```bash
# Scan current directory for venvs unused 30+ days (default)
tidyup

# Scan for everything
tidyup -all ~

# Scan for specific types
tidyup -type node_modules,pycache ~

# Scan home directory including uv caches
tidyup -system ~

# Scan multiple directories
tidyup ~/dev ~/projects ~/experiments

# Delete with interactive selection
tidyup -all -delete ~

# Preview what -delete would do without acting
tidyup -all -delete -dry-run ~

# JSON output for scripting
tidyup -all -json ~ | jq '.records[] | select(.type == "node_modules") | .path'
```

### Interactive Selection

When using `-delete` without `-confirm`, tidyup shows a numbered list and lets you pick:

```
   1. [venv]         1.2 GB   342d ago  /Users/fred/dev/myproject/.venv
   2. [node_modules]  450 MB  120d ago  /Users/fred/dev/website/node_modules
   3. [pycache]        12 MB   45d ago  /Users/fred/dev/scripts/__pycache__

Select items to PERMANENTLY DELETE (e.g., 1,3 or 1-3 or 'all' or 'none'):
```

Input formats: `1,3,5` (individual), `1-3` (range), `1-3,5` (mixed), `all`, `none`.

### macOS + uv Examples

```bash
# Find the largest stale environments
tidyup -system -age 90 -sort size ~

# Move to Trash instead of permanent delete
tidyup -all -delete -trash ~

# Non-interactive deletion for CI/automation
tidyup -all -delete -confirm -age 90 ~

# Log deletions for audit
tidyup -all -delete -log cleanup.log ~
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-age N` | `30` | Minimum days since last use |
| `-depth N` | `5` | Maximum scan recursion depth |
| `-delete` | `false` | Delete identified items (with interactive selection) |
| `-dry-run` | `false` | Preview deletions without acting (overrides `-delete`) |
| `-type T` | `venv` | Comma-separated types to scan for |
| `-all` | `false` | Scan for all supported types |
| `-system` | `false` | Include standard uv cache locations |
| `-json` | `false` | Machine-readable JSON output |
| `-verbose` | `false` | Show scan progress on stderr |
| `-exclude P` | | Comma-separated path patterns to skip |
| `-min-size N` | `0` | Only report items above N bytes |
| `-sort F` | `size` | Sort by: `size`, `age`, or `path` |
| `-trash` | `false` | Move to `~/.Trash` instead of permanent delete (macOS) |
| `-confirm` | `false` | Skip interactive selection (for CI/automation) |
| `-log FILE` | | Write timestamped deletion log to FILE |
| `-version` | | Print version and exit |

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | No stale items found |
| `1` | Stale items found (or deleted) |
| `2` | Error |

## Safety Features

- **Active venv protection**: If `$VIRTUAL_ENV` matches a detected venv, it is excluded from deletion with a warning.
- **Path guards**: System-critical paths (`/usr`, `/System`, `/Library`, `$HOME`, etc.) are never deleted.
- **Venv validation**: A `pyvenv.cfg` file alone is not enough -- requires `bin/` or `Scripts/` to avoid deleting project roots.
- **Improved staleness detection**: Checks site-packages for recent package installs, not just activation script timestamps.

## Technical Notes

- **Pruning**: Skips `.git`, `Library`, `.Trash` unconditionally. Skips `node_modules`, `__pycache__`, etc. when not scanning for those types.
- **Detection**: Venvs use content-based detection (pyvenv.cfg). All other types use directory name matching.
- **Build directories**: `dist/` and `build/` require a build system marker in the parent to avoid false positives on unrelated directories.
- **Permissions**: Ensure you have proper permissions for scanned directories.
- **Symlinks**: `filepath.WalkDir` does not follow symlinks.

## License

MIT -- see LICENSE file.
