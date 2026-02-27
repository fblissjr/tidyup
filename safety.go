package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// isActiveVenv returns true if path matches $VIRTUAL_ENV.
func isActiveVenv(path string) bool {
	venv := os.Getenv("VIRTUAL_ENV")
	if venv == "" {
		return false
	}
	// Clean both paths for reliable comparison.
	return filepath.Clean(path) == filepath.Clean(venv)
}

// protectedPrefixes are system-critical path prefixes that should never be deleted.
var protectedPrefixes = []string{
	"/usr",
	"/bin",
	"/sbin",
	"/etc",
	"/var",
	"/tmp",
	"/private",
	"/System",
	"/Library",
	"/Applications",
}

// isProtectedPath returns true if path is a system-critical location
// or an ancestor of (or equal to) $HOME.
func isProtectedPath(path string) bool {
	cleaned := filepath.Clean(path)

	for _, prefix := range protectedPrefixes {
		if cleaned == prefix || strings.HasPrefix(cleaned, prefix+"/") {
			return true
		}
	}

	// Protect $HOME and its ancestors.
	home, err := os.UserHomeDir()
	if err == nil {
		home = filepath.Clean(home)
		// path is protected if it IS home or IS an ancestor of home.
		if cleaned == home {
			return true
		}
		if strings.HasPrefix(home, cleaned+"/") {
			return true
		}
	}

	return false
}

// isValidVenv returns true if the directory looks like a real venv
// (has pyvenv.cfg AND bin/ or Scripts/ directory).
func isValidVenv(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "pyvenv.cfg")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "bin")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(path, "Scripts")); err == nil {
		return true
	}
	return false
}

// getSitePackagesUsage checks site-packages for the newest mtime among
// installed packages, providing a better "last used" signal than activation
// script timestamps alone.
func getSitePackagesUsage(path string) (time.Time, bool) {
	var latest time.Time
	found := false

	// Look for lib/python*/site-packages pattern.
	pattern := filepath.Join(path, "lib", "python*", "site-packages")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return latest, false
	}

	for _, spDir := range matches {
		_ = filepath.WalkDir(spDir, func(p string, d fs.DirEntry, err error) error {
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
	}

	return latest, found
}
