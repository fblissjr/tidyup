// tidyup -- lightweight environment and cache cleaner.
// Finds and removes unused Python venvs, node_modules, caches, and build artifacts.

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// version is set at build time via -ldflags.
var version = "dev"

// Exit codes for scripting.
const (
	exitOK    = 0
	exitFound = 1
	exitError = 2
)

// allScanTypes lists every type tidyup knows how to detect.
var allScanTypes = []string{
	"venv", "node_modules", "pycache", "pytest_cache",
	"mypy_cache", "ruff_cache", "dist", "build",
}

// options holds all parsed CLI flags.
type options struct {
	minAge          int
	maxDepth        int
	doDelete        bool
	dryRun          bool
	systemScan      bool
	jsonOut         bool
	verbose         bool
	excludePatterns []string
	minSize         int64
	sortField       string
	useTrash        bool
	logFile         string
	confirm         bool
	scanTypes       map[string]bool
}

// parseScanTypes converts the --type flag and --all flag into a type map.
// Returns the map and any warnings for unrecognized type values.
func parseScanTypes(typeFlag string, allTypes bool) (map[string]bool, []string) {
	if allTypes {
		m := make(map[string]bool, len(allScanTypes))
		for _, t := range allScanTypes {
			m[t] = true
		}
		return m, nil
	}

	if typeFlag == "" {
		return map[string]bool{"venv": true}, nil
	}

	valid := make(map[string]bool, len(allScanTypes))
	for _, t := range allScanTypes {
		valid[t] = true
	}

	m := make(map[string]bool)
	var warnings []string
	for _, raw := range strings.Split(typeFlag, ",") {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if valid[t] {
			m[t] = true
		} else {
			warnings = append(warnings, fmt.Sprintf("unrecognized type %q (known: %s)", t, strings.Join(allScanTypes, ", ")))
		}
	}

	// If all provided types were invalid, fall back to default.
	if len(m) == 0 {
		m["venv"] = true
	}

	return m, warnings
}

func main() {
	os.Exit(run())
}

func run() int {
	// Flags.
	minAge := flag.Int("age", 30, "Min days since last use")
	maxDepth := flag.Int("depth", 5, "Scan depth for recursion")
	doDelete := flag.Bool("delete", false, "Delete the identified items")
	dryRun := flag.Bool("dry-run", false, "Preview what would be deleted (overrides -delete)")
	systemScan := flag.Bool("system", false, "Include standard uv cache locations (~/.local/share/uv)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	jsonOut := flag.Bool("json", false, "Output results as JSON")
	verbose := flag.Bool("verbose", false, "Show scan progress on stderr")
	excludeRaw := flag.String("exclude", "", "Comma-separated path patterns to skip")
	minSize := flag.Int64("min-size", 0, "Only report items above this size in bytes")
	sortField := flag.String("sort", "size", "Sort by: size, age, path")
	useTrash := flag.Bool("trash", false, "Move to ~/.Trash instead of permanent delete (macOS)")
	logFile := flag.String("log", "", "Write deletion log to this file")
	confirm := flag.Bool("confirm", false, "Skip interactive selection prompt (for automation)")
	typeFlag := flag.String("type", "", "Comma-separated types: venv,node_modules,pycache,pytest_cache,mypy_cache,ruff_cache,dist,build")
	allTypes := flag.Bool("all", false, "Scan for all supported types")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "tidyup: Locates and cleans up unused environments, caches, and build artifacts.\n\n")
		fmt.Fprintf(os.Stderr, "Usage: tidyup [flags] [paths...]\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSupported types: %s\n", strings.Join(allScanTypes, ", "))
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  tidyup                                Scan current dir, venvs unused 30+ days\n")
		fmt.Fprintf(os.Stderr, "  tidyup -system ~                      Scan home + uv caches\n")
		fmt.Fprintf(os.Stderr, "  tidyup -age 60 -delete ~              Delete venvs unused 60+ days\n")
		fmt.Fprintf(os.Stderr, "  tidyup -delete -dry-run ~             Preview deletions without acting\n")
		fmt.Fprintf(os.Stderr, "  tidyup -delete -confirm -trash ~      Auto-confirm, move to Trash\n")
		fmt.Fprintf(os.Stderr, "  tidyup -json -system ~                Machine-readable output\n")
		fmt.Fprintf(os.Stderr, "  tidyup ~/dev ~/projects               Scan multiple directories\n")
		fmt.Fprintf(os.Stderr, "  tidyup -all ~                         Scan for everything\n")
		fmt.Fprintf(os.Stderr, "  tidyup -type node_modules,pycache ~   Scan for specific types\n")
		fmt.Fprintf(os.Stderr, "  tidyup -all -delete -trash ~          Clean all types, move to Trash\n")
		fmt.Fprintf(os.Stderr, "\nExit codes: 0=nothing found, 1=stale items found, 2=error\n")
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("tidyup %s\n", version)
		return exitOK
	}

	// Parse scan types.
	scanTypes, typeWarnings := parseScanTypes(*typeFlag, *allTypes)
	for _, w := range typeWarnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
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

	// --dry-run overrides --delete.
	if *dryRun {
		*doDelete = false
	}

	opts := &options{
		minAge:          *minAge,
		maxDepth:        *maxDepth,
		doDelete:        *doDelete,
		dryRun:          *dryRun,
		systemScan:      *systemScan,
		jsonOut:         *jsonOut,
		verbose:         *verbose,
		excludePatterns: excludePatterns,
		minSize:         *minSize,
		sortField:       *sortField,
		useTrash:        *useTrash,
		logFile:         *logFile,
		confirm:         *confirm,
		scanTypes:       scanTypes,
	}

	// Collect root paths.
	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}

	// Include standard uv locations (only relevant when scanning venvs).
	if opts.systemScan {
		if !opts.scanTypes["venv"] {
			fmt.Fprintf(os.Stderr, "Warning: -system only adds uv cache paths for venv scanning; ignored for other types.\n")
		} else {
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
	}

	if opts.verbose {
		fmt.Fprintf(os.Stderr, "Scanning roots: %v\n", roots)
		var typeNames []string
		for _, t := range allScanTypes {
			if opts.scanTypes[t] {
				typeNames = append(typeNames, t)
			}
		}
		fmt.Fprintf(os.Stderr, "Scanning for types: %v\n", typeNames)
	}

	// Scan.
	records, scanErrors := scanRoots(roots, opts)

	for _, e := range scanErrors {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", e)
	}

	// Sort and total.
	sortRecords(records, opts.sortField)
	total := totalSize(records)

	// Output.
	if opts.jsonOut {
		return printJSON(records, total, !opts.doDelete)
	}

	printText(records, total)

	if len(records) == 0 {
		return exitOK
	}

	// Deletion.
	if opts.doDelete {
		return deleteRecords(records, opts)
	}

	fmt.Println("\nRun with '-delete' to reclaim this space.")
	return exitFound
}
