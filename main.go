// main.go
// tidyup -- lightweight Python virtual environment locator and cleaner.
// Optimized for macOS and 'uv' environments with advanced activity detection.

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// version is set at build time via -ldflags.
var version = "dev"

// Exit codes for scripting.
const (
	exitOK       = 0
	exitFound    = 1
	exitError    = 2
)

// VenvRecord holds metadata about a found environment for evaluation.
type VenvRecord struct {
	Path     string  `json:"path"`
	Size     int64   `json:"size_bytes"`
	SizeHuman string `json:"size_human"`
	LastUsed string  `json:"last_used"`
	AgeDays  float64 `json:"age_days"`
}

// JSONOutput is the top-level structure for --json output.
type JSONOutput struct {
	Count      int          `json:"count"`
	TotalBytes int64        `json:"total_bytes"`
	TotalHuman string       `json:"total_human"`
	Records    []VenvRecord `json:"records"`
	DryRun     bool         `json:"dry_run"`
}

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

func main() {
	os.Exit(run())
}

func run() int {
	// Flags.
	minAge := flag.Int("age", 30, "Min days since last use")
	maxDepth := flag.Int("depth", 5, "Scan depth for recursion")
	doDelete := flag.Bool("delete", false, "Delete the identified environments")
	dryRun := flag.Bool("dry-run", false, "Preview what would be deleted (default behavior, useful with -delete to override)")
	systemScan := flag.Bool("system", false, "Include standard uv cache locations (~/.local/share/uv)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	jsonOut := flag.Bool("json", false, "Output results as JSON")
	verbose := flag.Bool("verbose", false, "Show scan progress on stderr")
	excludeRaw := flag.String("exclude", "", "Comma-separated path patterns to skip")
	minSize := flag.Int64("min-size", 0, "Only report venvs above this size in bytes")
	sortField := flag.String("sort", "size", "Sort by: size, age, path")
	useTrash := flag.Bool("trash", false, "Move to ~/.Trash instead of permanent delete (macOS)")
	logFile := flag.String("log", "", "Write deletion log to this file")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "tidyup: Locates and cleans up unused Python environments.\n\n")
		fmt.Fprintf(os.Stderr, "Usage: tidyup [flags] [paths...]\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  tidyup                          Scan current dir for venvs unused 30+ days\n")
		fmt.Fprintf(os.Stderr, "  tidyup -system ~                Scan home + uv caches\n")
		fmt.Fprintf(os.Stderr, "  tidyup -age 60 -delete ~        Delete venvs unused 60+ days\n")
		fmt.Fprintf(os.Stderr, "  tidyup -delete -dry-run ~       Preview deletions without acting\n")
		fmt.Fprintf(os.Stderr, "  tidyup -json -system ~          Machine-readable output\n")
		fmt.Fprintf(os.Stderr, "  tidyup ~/dev ~/projects         Scan multiple directories\n")
		fmt.Fprintf(os.Stderr, "\nExit codes: 0=nothing found, 1=stale venvs found, 2=error\n")
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("tidyup %s\n", version)
		return exitOK
	}

	// Parse exclude patterns.
	var excludePatterns []string
	if *excludeRaw != "" {
		for _, p := range strings.Split(*excludeRaw, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				excludePatterns = append(excludePatterns, trimmed)
			}
		}
	}

	// If --dry-run is set alongside --delete, it cancels the delete (preview only).
	if *dryRun {
		*doDelete = false
	}

	// Collect root paths -- accept multiple positional args.
	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}

	// Include standard uv locations.
	if *systemScan {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not determine home directory: %v\n", err)
			return exitError
		}
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
	var scanErrors []string
	var scanned int64

	if *verbose {
		fmt.Fprintf(os.Stderr, "Scanning roots: %v\n", roots)
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
				if depth > *maxDepth {
					return filepath.SkipDir
				}
			}

			// Skip system noise.
			switch d.Name() {
			case ".git", "node_modules", "Library", ".Trash", "__pycache__":
				return filepath.SkipDir
			}

			// Exclude patterns.
			if matchesExclude(path, excludePatterns) {
				return filepath.SkipDir
			}

			if isVenv(path) {
				lastUsed, found := getVenvUsage(path)
				if !found {
					if *verbose {
						fmt.Fprintf(os.Stderr, "  skipping (no markers): %s\n", path)
					}
					return filepath.SkipDir
				}

				age := time.Since(lastUsed).Hours() / 24

				if age >= float64(*minAge) {
					wg.Add(1)
					go func(p string, lu time.Time, ad float64) {
						defer wg.Done()
						sz := dirSize(p)
						if sz < *minSize {
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
						if *verbose {
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

	// Report any scan errors.
	for _, e := range scanErrors {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", e)
	}

	// Sort results.
	switch *sortField {
	case "age":
		sort.Slice(records, func(i, j int) bool {
			return records[i].AgeDays > records[j].AgeDays
		})
	case "path":
		sort.Slice(records, func(i, j int) bool {
			return records[i].Path < records[j].Path
		})
	default: // "size"
		sort.Slice(records, func(i, j int) bool {
			return records[i].Size > records[j].Size
		})
	}

	var totalSize int64
	for _, r := range records {
		totalSize += r.Size
	}

	// JSON output mode.
	if *jsonOut {
		out := JSONOutput{
			Count:      len(records),
			TotalBytes: totalSize,
			TotalHuman: formatBytes(totalSize),
			Records:    records,
			DryRun:     !*doDelete,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			return exitError
		}
		if len(records) == 0 {
			return exitOK
		}
		return exitFound
	}

	// Text output.
	for _, r := range records {
		fmt.Printf("%-10s %-4.0fd ago  %s\n", r.SizeHuman, r.AgeDays, r.Path)
	}

	if len(records) == 0 {
		fmt.Println("No unused environments found.")
		return exitOK
	}

	fmt.Printf("\nFound %d environments totaling %s\n", len(records), formatBytes(totalSize))

	// Deletion logic.
	if *doDelete {
		// Open log file if requested.
		var logWriter *os.File
		if *logFile != "" {
			var err error
			logWriter, err = os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
				return exitError
			}
			defer logWriter.Close()
			fmt.Fprintf(logWriter, "# tidyup deletion log -- %s\n", time.Now().Format(time.RFC3339))
		}

		if *useTrash && runtime.GOOS == "darwin" {
			fmt.Printf("\nWARNING: You are about to move %d folders to Trash. Continue? [y/N]: ", len(records))
		} else {
			fmt.Printf("\nWARNING: You are about to PERMANENTLY DELETE %d folders. Continue? [y/N]: ", len(records))
		}

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(response)) != "y" {
			fmt.Println("Cleanup cancelled.")
			return exitFound
		}

		var deletedCount int
		for _, r := range records {
			var err error
			if *useTrash && runtime.GOOS == "darwin" {
				trashPath := filepath.Join(os.Getenv("HOME"), ".Trash", filepath.Base(r.Path))
				err = os.Rename(r.Path, trashPath)
			} else {
				err = os.RemoveAll(r.Path)
			}

			if err == nil {
				action := "Deleted"
				if *useTrash && runtime.GOOS == "darwin" {
					action = "Trashed"
				}
				fmt.Printf("%s: %s\n", action, r.Path)
				deletedCount++
				if logWriter != nil {
					fmt.Fprintf(logWriter, "%s %s %s %s\n",
						time.Now().Format(time.RFC3339), action, formatBytes(r.Size), r.Path)
				}
			} else {
				fmt.Fprintf(os.Stderr, "Error removing %s: %v\n", r.Path, err)
				if logWriter != nil {
					fmt.Fprintf(logWriter, "%s ERROR %s: %v\n",
						time.Now().Format(time.RFC3339), r.Path, err)
				}
			}
		}
		fmt.Printf("\nCleanup complete. Removed %d environments.\n", deletedCount)
	} else {
		fmt.Println("\nRun with '-delete' to reclaim this space.")
	}

	return exitFound
}
