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

// getNodeModulesUsage determines when a node_modules directory was last used.
// Checks .package-lock.json (npm >=7), parent lockfiles, then falls back to dir mtime.
func getNodeModulesUsage(path string) (time.Time, bool) {
	// Check .package-lock.json inside node_modules (npm >=7 writes this on install).
	if info, err := os.Stat(filepath.Join(path, ".package-lock.json")); err == nil {
		return info.ModTime(), true
	}

	// Fallback: check parent directory lockfiles.
	parent := filepath.Dir(path)
	for _, name := range []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml", "bun.lockb"} {
		if info, err := os.Stat(filepath.Join(parent, name)); err == nil {
			return info.ModTime(), true
		}
	}

	// Fallback: directory mtime.
	if info, err := os.Stat(path); err == nil {
		return info.ModTime(), true
	}
	return time.Time{}, false
}

// getCacheUsage walks a cache directory to find the newest file mtime.
// Shared by pycache, pytest_cache, mypy_cache, ruff_cache.
func getCacheUsage(path string) (time.Time, bool) {
	var latest time.Time
	found := false

	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				found = true
				if mtime := info.ModTime(); mtime.After(latest) {
					latest = mtime
				}
			}
		}
		return nil
	})

	if !found {
		// Fallback: directory mtime.
		if info, err := os.Stat(path); err == nil {
			return info.ModTime(), true
		}
	}

	return latest, found
}

// getBuildUsage finds the newest file mtime in a build/dist directory.
func getBuildUsage(path string) (time.Time, bool) {
	return getCacheUsage(path) // Same logic: newest file or dir mtime.
}

// hasBuildParent returns true if the parent directory contains build system markers.
// Required for dist/ and build/ since those names are too generic on their own.
func hasBuildParent(path string) bool {
	parent := filepath.Dir(path)
	markers := []string{"pyproject.toml", "setup.py", "setup.cfg", "package.json"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(parent, m)); err == nil {
			return true
		}
	}
	return false
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

// usageFunc is the signature for type-specific usage heuristic functions.
type usageFunc func(string) (time.Time, bool)

// dispatchRecord calculates size and usage for a detected item and appends a Record.
func dispatchRecord(path, typeName string, usage usageFunc,
	opts *options, wg *sync.WaitGroup, mu *sync.Mutex, records *[]Record, scanned *int64) {

	lastUsed, found := usage(path)
	if !found {
		return
	}

	age := time.Since(lastUsed).Hours() / 24
	if age < float64(opts.minAge) {
		return
	}

	wg.Add(1)
	go func(p string, lu time.Time, ad float64) {
		defer wg.Done()
		sz := dirSize(p)
		if sz < opts.minSize {
			return
		}
		mu.Lock()
		*records = append(*records, Record{
			Type:      typeName,
			Path:      p,
			Size:      sz,
			SizeHuman: formatBytes(sz),
			LastUsed:  lu.Format("2006-01-02"),
			AgeDays:   ad,
		})
		mu.Unlock()
		if opts.verbose {
			mu.Lock()
			*scanned++
			fmt.Fprintf(os.Stderr, "  found %d stale items so far...\r", *scanned)
			mu.Unlock()
		}
	}(path, lastUsed, age)
}

// scanRoots walks all root directories and returns matching Records.
func scanRoots(roots []string, opts *options) ([]Record, []string) {
	var records []Record
	var mu sync.Mutex
	var wg sync.WaitGroup
	var scanErrors []string
	var scanned int64

	// Map directory names to their scan type keys and skip behavior.
	// If we're scanning for the type, detect+dispatch. Otherwise, skip.
	skipUnlessScanning := map[string]string{
		"node_modules":  "node_modules",
		"__pycache__":   "pycache",
		".pytest_cache": "pytest_cache",
		".mypy_cache":   "mypy_cache",
		".ruff_cache":   "ruff_cache",
	}

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

			// Always skip these.
			switch d.Name() {
			case ".git", "Library", ".Trash":
				return filepath.SkipDir
			}

			// Exclude patterns.
			if matchesExclude(path, opts.excludePatterns) {
				return filepath.SkipDir
			}

			name := d.Name()

			// Unified name-based detection and skip logic.
			if typeKey, ok := skipUnlessScanning[name]; ok {
				if opts.scanTypes[typeKey] {
					var fn usageFunc
					switch typeKey {
					case "node_modules":
						fn = getNodeModulesUsage
					default:
						fn = getCacheUsage
					}
					dispatchRecord(path, typeKey, fn, opts, &wg, &mu, &records, &scanned)
				}
				return filepath.SkipDir
			}

			// dist/ and build/ -- require parent validation.
			if name == "dist" {
				if opts.scanTypes["dist"] && hasBuildParent(path) {
					dispatchRecord(path, "dist", getBuildUsage, opts, &wg, &mu, &records, &scanned)
					return filepath.SkipDir
				}
				// Don't skip -- could be a normal directory.
			}
			if name == "build" {
				if opts.scanTypes["build"] && hasBuildParent(path) {
					dispatchRecord(path, "build", getBuildUsage, opts, &wg, &mu, &records, &scanned)
					return filepath.SkipDir
				}
			}

			// Content-based detection: venv (needs file check).
			if opts.scanTypes["venv"] && isVenv(path) {
				if !isValidVenv(path) {
					if opts.verbose {
						fmt.Fprintf(os.Stderr, "  skipping (invalid venv, no bin/Scripts): %s\n", path)
					}
					return filepath.SkipDir
				}

				lastUsed, found := getVenvUsage(path)
				if !found {
					if opts.verbose {
						fmt.Fprintf(os.Stderr, "  skipping (no markers): %s\n", path)
					}
					return filepath.SkipDir
				}

				// Check site-packages for a more recent usage signal.
				if spTime, ok := getSitePackagesUsage(path); ok && spTime.After(lastUsed) {
					lastUsed = spTime
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
						records = append(records, Record{
							Type:      "venv",
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
							fmt.Fprintf(os.Stderr, "  found %d stale items so far...\r", scanned)
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
