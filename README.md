# tidyup

Lightweight Go CLI to identify and clean up unused Python virtual environments. Optimized for macOS and `uv` users who accumulate stale environments.

## Features

- **Advanced Activity Detection** -- Inspects `pyvenv.cfg` and activation scripts for actual usage timestamps rather than unreliable directory access times. Falls back to directory mtime when markers are missing.
- **Concurrent Scanning** -- Uses goroutines to calculate directory sizes in parallel.
- **System-Wide Awareness** -- With `--system`, automatically includes standard uv cache locations (`~/Library/Caches/uv/venvs`, etc.).
- **Safe Deletion** -- Interactive confirmation prompt, optional `--dry-run`, optional `--trash` (macOS) to move to Trash instead of permanent delete.
- **Machine-Readable Output** -- `--json` flag for scripting and piping to `jq`.
- **Auditable** -- `--log` writes a timestamped deletion log.

## Installation

```bash
git clone https://github.com/fblissjr/tidyup.git
cd tidyup
make install
```

This builds the binary and moves it to `/usr/local/bin` (requires sudo).

## Usage

### Quick Start

```bash
# Scan current directory for venvs unused 30+ days
tidyup

# Scan home directory including uv caches
tidyup -system ~

# Scan multiple directories
tidyup ~/dev ~/projects ~/experiments

# Delete venvs unused 60+ days
tidyup -age 60 -delete ~

# Preview what -delete would do without acting
tidyup -delete -dry-run ~

# JSON output for scripting
tidyup -json -system ~ | jq '.records[] | .path'
```

### macOS + uv Examples

```bash
# Preview all global uv environments unused for 2 months
tidyup -system -age 60 ~

# Find the largest stale environments
tidyup -system -age 90 -sort size ~

# Move to Trash instead of permanent delete
tidyup -delete -trash -system ~

# Non-interactive deletion for CI/automation
tidyup -delete -confirm -age 90 -system ~

# Log deletions for audit
tidyup -delete -log cleanup.log -system ~
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-age N` | `30` | Minimum days since last use |
| `-depth N` | `5` | Maximum scan recursion depth |
| `-delete` | `false` | Delete identified environments |
| `-dry-run` | `false` | Preview deletions without acting (overrides `-delete`) |
| `-system` | `false` | Include standard uv cache locations |
| `-json` | `false` | Machine-readable JSON output |
| `-verbose` | `false` | Show scan progress on stderr |
| `-exclude P` | | Comma-separated path patterns to skip |
| `-min-size N` | `0` | Only report venvs above N bytes |
| `-sort F` | `size` | Sort by: `size`, `age`, or `path` |
| `-trash` | `false` | Move to `~/.Trash` instead of permanent delete (macOS) |
| `-confirm` | `false` | Skip interactive y/N prompt (for CI/automation) |
| `-log FILE` | | Write timestamped deletion log to FILE |
| `-version` | | Print version and exit |

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | No stale environments found |
| `1` | Stale environments found (or deleted) |
| `2` | Error |

## Technical Notes

- **Pruning**: Aggressively skips `.git`, `node_modules`, `Library`, `.Trash`, and `__pycache__` directories for performance.
- **Detection**: Uses `pyvenv.cfg` as the venv marker. Falls back to directory mtime if activation scripts are missing.
- **Permissions**: Ensure you have proper permissions for scanned directories.
- **Symlinks**: `filepath.WalkDir` does not follow symlinks.

## License

MIT -- see LICENSE file.
