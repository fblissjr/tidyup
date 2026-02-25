// main.go
// tidyup – lightweight Python virtual environment locator and cleaner.
// Optimized for macOS and 'uv' environments with advanced activity detection.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// VenvRecord holds metadata about a found environment for evaluation.
type VenvRecord struct {
	Path     string
	Size     int64
	LastUsed time.Time
	AgeDays  float64
}

// getVenvUsage inspects specific venv markers to determine the last time it was actually "used".
// This is more reliable than directory timestamps on APFS.
func getVenvUsage(path string) time.Time {
	binDir := "bin"
	if runtime.GOOS == "windows" {
		binDir = "Scripts"
	}

	// We look for activity in activation scripts and the pyvenv.cfg.
	targets := []string{
		filepath.Join(path, binDir, "activate"),
		filepath.Join(path, "pyvenv.cfg"),
		filepath.Join(path, binDir, "python"),
	}

	var latest time.Time
	for _, t := range targets {
		if info, err := os.Stat(t); err == nil {
			atime := info.ModTime() // Using ModTime as it's the most consistent across OSes.
			if atime.After(latest) {
				latest = atime
			}
		}
	}
	return latest
}

// isVenv identifies if a directory is a Python virtual environment via the pyvenv.cfg marker.
func isVenv(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "pyvenv.cfg")); err != nil {
		return false
	}
	return true
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

// formatBytes provides human-readable output (MB, GB, etc.)
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func main() {
	minAge := flag.Int("age", 30, "Min days since last use (default: 30)")
	maxDepth := flag.Int("depth", 5, "Scan depth for recursion")
	doDelete := flag.Bool("delete", false, "Actually delete the identified environments")
	systemScan := flag.Bool("system", false, "Include standard uv cache locations (~/.local/share/uv)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "tidyup: Locates and cleans up unused Python environments.\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample: tidyup -age 60 -system -delete ~\n")
	}
	flag.Parse()

	roots := []string{"."}
	if flag.NArg() > 0 {
		roots = []string{flag.Arg(0)}
	}

	// Incorporating standard uv locations from the Python project.
	if *systemScan {
		home, _ := os.UserHomeDir()
		extra := []string{
			filepath.Join(home, ".local/share/uv/venvs"),
			filepath.Join(home, "Library/Caches/uv/venvs"),
		}
		for _, p := range extra {
			if _, err := os.Stat(p); err == nil {
				roots = append(roots, p)
			}
		}
	}

	var records []VenvRecord
	var mu sync.Mutex
	var wg sync.WaitGroup

	fmt.Fprintf(os.Stderr, "Scanning roots: %v\n", roots)

	for _, root := range roots {
		absRoot, _ := filepath.Abs(root)
		_ = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}

			// Pruning depth and system noise.
			rel, _ := filepath.Rel(absRoot, path)
			if rel != "." {
				depth := strings.Count(filepath.ToSlash(rel), "/") + 1
				if depth > *maxDepth {
					return filepath.SkipDir
				}
			}

			switch d.Name() {
			case ".git", "node_modules", "Library", ".Trash", "__pycache__":
				return filepath.SkipDir
			}

			if isVenv(path) {
				lastUsed := getVenvUsage(path)
				age := time.Since(lastUsed).Hours() / 24

				if age >= float64(*minAge) {
					wg.Add(1)
					go func(p string, lu time.Time, ad float64) {
						defer wg.Done()
						sz := dirSize(p)
						mu.Lock()
						records = append(records, VenvRecord{
							Path:     p,
							Size:     sz,
							LastUsed: lu,
							AgeDays:  ad,
						})
						mu.Unlock()
					}(path, lastUsed, age)
				}
				// venvs don't usually nest other venvs.
				return filepath.SkipDir
			}
			return nil
		})
	}

	wg.Wait()

	// Sort largest folders to the top.
	sort.Slice(records, func(i, j int) bool {
		return records[i].Size > records[j].Size
	})

	var totalSize int64
	for _, r := range records {
		fmt.Printf("%-10s %-4.0fd ago  %s\n", formatBytes(r.Size), r.AgeDays, r.Path)
		totalSize += r.Size
	}

	if len(records) == 0 {
		fmt.Println("No unused environments found.")
		return
	}

	fmt.Printf("\nFound %d environments totaling %s\n", len(records), formatBytes(totalSize))

	// Interactive safe-delete logic.
	if *doDelete {
		fmt.Printf("\nWARNING: You are about to PERMANENTLY DELETE these folders. Continue? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(response)) == "y" {
			var deletedCount int32
			for _, r := range records {
				err := os.RemoveAll(r.Path)
				if err == nil {
					fmt.Printf("Deleted: %s\n", r.Path)
					atomic.AddInt32(&deletedCount, 1)
				} else {
					fmt.Printf("Error deleting %s: %v\n", r.Path, err)
				}
			}
			fmt.Printf("\nCleanup complete. Removed %d environments.\n", deletedCount)
		} else {
			fmt.Println("Cleanup cancelled.")
		}
	} else {
		fmt.Println("\nRun with '-delete' to reclaim this space.")
	}
}
