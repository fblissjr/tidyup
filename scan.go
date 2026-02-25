package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// getVenvUsage inspects specific venv markers to determine the last time it was actually "used".
// Returns the latest mtime found and whether any marker was found at all.
func getVenvUsage(path string) (time.Time, bool) {
	binDir := "bin"
	if runtime.GOOS == "windows" {
		binDir = "Scripts"
	}

	targets := []string{
		filepath.Join(path, binDir, "activate"),
		filepath.Join(path, "pyvenv.cfg"),
		filepath.Join(path, binDir, "python"),
	}

	var latest time.Time
	found := false
	for _, t := range targets {
		if info, err := os.Stat(t); err == nil {
			found = true
			if mtime := info.ModTime(); mtime.After(latest) {
				latest = mtime
			}
		}
	}

	// Fall back to the venv directory's own mtime if no markers were readable.
	if !found {
		if info, err := os.Stat(path); err == nil {
			return info.ModTime(), true
		}
	}

	return latest, found
}

// isVenv identifies if a directory is a Python virtual environment via the pyvenv.cfg marker.
func isVenv(path string) bool {
	_, err := os.Stat(filepath.Join(path, "pyvenv.cfg"))
	return err == nil
}

// dirSize recursively calculates total bytes in a directory.
func dirSize(path string) int64 {
	var size int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}

// matchesExclude checks if a path matches any of the exclude patterns.
func matchesExclude(path string, patterns []string) bool {
	for _, pat := range patterns {
		if matched, _ := filepath.Match(pat, filepath.Base(path)); matched {
			return true
		}
		if strings.Contains(path, pat) {
			return true
		}
	}
	return false
}

// scanRoots walks all root directories and returns matching VenvRecords.
func scanRoots(roots []string, opts *options) ([]VenvRecord, []string) {
	var records []VenvRecord
	var mu sync.Mutex
	var wg sync.WaitGroup
	var scanErrors []string
	var scanned int64

	for _, root := range roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			scanErrors = append(scanErrors, fmt.Sprintf("bad path %q: %v", root, err))
			continue
		}

		if _, err := os.Stat(absRoot); err != nil {
			scanErrors = append(scanErrors, fmt.Sprintf("path not accessible %q: %v", absRoot, err))
			continue
		}

		_ = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}

			// Depth pruning.
			rel, _ := filepath.Rel(absRoot, path)
			if rel != "." {
				depth := strings.Count(filepath.ToSlash(rel), "/") + 1
				if depth > opts.maxDepth {
					return filepath.SkipDir
				}
			}

			// Skip system noise.
			switch d.Name() {
			case ".git", "node_modules", "Library", ".Trash", "__pycache__":
				return filepath.SkipDir
			}

			// Exclude patterns.
			if matchesExclude(path, opts.excludePatterns) {
				return filepath.SkipDir
			}

			if isVenv(path) {
				lastUsed, found := getVenvUsage(path)
				if !found {
					if opts.verbose {
						fmt.Fprintf(os.Stderr, "  skipping (no markers): %s\n", path)
					}
					return filepath.SkipDir
				}

				age := time.Since(lastUsed).Hours() / 24

				if age >= float64(opts.minAge) {
					wg.Add(1)
					go func(p string, lu time.Time, ad float64) {
						defer wg.Done()
						sz := dirSize(p)
						if sz < opts.minSize {
							return
						}
						mu.Lock()
						records = append(records, VenvRecord{
							Path:      p,
							Size:      sz,
							SizeHuman: formatBytes(sz),
							LastUsed:  lu.Format("2006-01-02"),
							AgeDays:   ad,
						})
						mu.Unlock()
						if opts.verbose {
							mu.Lock()
							scanned++
							fmt.Fprintf(os.Stderr, "  found %d stale venvs so far...\r", scanned)
							mu.Unlock()
						}
					}(path, lastUsed, age)
				}
				return filepath.SkipDir
			}
			return nil
		})
	}

	wg.Wait()
	return records, scanErrors
}
