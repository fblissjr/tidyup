package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// moveToTrash moves a path to ~/.Trash with collision-safe naming.
// Appends a timestamp suffix if the basename already exists in Trash.
func moveToTrash(path string) error {
	home := os.Getenv("HOME")
	if home == "" {
		return fmt.Errorf("HOME not set")
	}

	trashDir := filepath.Join(home, ".Trash")
	base := filepath.Base(path)
	dest := filepath.Join(trashDir, base)

	// If destination already exists, append a timestamp to avoid collision.
	if _, err := os.Stat(dest); err == nil {
		stamp := time.Now().Format("20060102-150405")
		dest = filepath.Join(trashDir, fmt.Sprintf("%s_%s", base, stamp))
	}

	return os.Rename(path, dest)
}

// parseSelection parses user input like "1,3,5-8" into a set of 0-based indices.
// Input uses 1-based numbering. max is the total number of items available.
func parseSelection(input string, max int) (map[int]bool, error) {
	input = strings.TrimSpace(input)
	if input == "" || strings.ToLower(input) == "none" {
		return map[int]bool{}, nil
	}
	if strings.ToLower(input) == "all" {
		result := make(map[int]bool, max)
		for i := 0; i < max; i++ {
			result[i] = true
		}
		return result, nil
	}

	result := make(map[int]bool)
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid number %q", bounds[0])
			}
			hi, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid number %q", bounds[1])
			}
			if lo < 1 || hi < 1 {
				return nil, fmt.Errorf("indices must be >= 1, got %d-%d", lo, hi)
			}
			if lo > hi {
				return nil, fmt.Errorf("invalid range %d-%d (start > end)", lo, hi)
			}
			if hi > max {
				return nil, fmt.Errorf("index %d out of range (max %d)", hi, max)
			}
			for i := lo; i <= hi; i++ {
				result[i-1] = true
			}
		} else {
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid input %q", part)
			}
			if n < 1 {
				return nil, fmt.Errorf("index must be >= 1, got %d", n)
			}
			if n > max {
				return nil, fmt.Errorf("index %d out of range (max %d)", n, max)
			}
			result[n-1] = true
		}
	}
	return result, nil
}

// promptSelection shows numbered records and returns the user-selected subset.
// Returns nil if the user cancels.
func promptSelection(records []Record, opts *options) []Record {
	fmt.Println()
	for i, r := range records {
		fmt.Printf("  %2d. %-12s %-10s %4.0fd ago  %s\n", i+1, "["+r.Type+"]", r.SizeHuman, r.AgeDays, r.Path)
	}
	fmt.Println()

	action := "PERMANENTLY DELETE"
	if opts.useTrash {
		action = "move to Trash"
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Select items to %s (e.g., 1,3 or 1-3 or 'all' or 'none'): ", action)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)

		selected, err := parseSelection(response, len(records))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid selection: %v. Try again.\n", err)
			continue
		}

		if len(selected) == 0 {
			return nil
		}

		var result []Record
		for i, r := range records {
			if selected[i] {
				result = append(result, r)
			}
		}
		return result
	}
}

// filterSafeRecords removes records that fail safety checks (active venv, protected paths).
// Returns the safe subset and prints warnings for filtered-out records.
func filterSafeRecords(records []Record) []Record {
	var safe []Record
	for _, r := range records {
		if isActiveVenv(r.Path) {
			fmt.Fprintf(os.Stderr, "Warning: skipping active venv ($VIRTUAL_ENV): %s\n", r.Path)
			continue
		}
		if isProtectedPath(r.Path) {
			fmt.Fprintf(os.Stderr, "Warning: skipping protected path: %s\n", r.Path)
			continue
		}
		safe = append(safe, r)
	}
	return safe
}

// deleteRecords handles the interactive or confirmed deletion of records.
func deleteRecords(records []Record, opts *options) int {
	// Validate --trash on non-macOS.
	if opts.useTrash && runtime.GOOS != "darwin" {
		fmt.Fprintf(os.Stderr, "Warning: -trash is only supported on macOS. Using permanent delete.\n")
		opts.useTrash = false
	}

	// Safety filtering before any user interaction.
	records = filterSafeRecords(records)
	if len(records) == 0 {
		fmt.Println("No safe records to delete after safety checks.")
		return exitOK
	}

	// Open log file if requested.
	var logWriter *os.File
	if opts.logFile != "" {
		var err error
		logWriter, err = os.OpenFile(opts.logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			return exitError
		}
		defer logWriter.Close()
		fmt.Fprintf(logWriter, "# tidyup deletion log -- %s\n", time.Now().Format(time.RFC3339))
	}

	// Interactive selection unless --confirm is set.
	if !opts.confirm {
		selected := promptSelection(records, opts)
		if selected == nil || len(selected) == 0 {
			fmt.Println("Cleanup cancelled.")
			return exitFound
		}
		records = selected
	}

	var deletedCount int
	for _, r := range records {
		var err error
		if opts.useTrash {
			err = moveToTrash(r.Path)
		} else {
			err = os.RemoveAll(r.Path)
		}

		if err == nil {
			action := "Deleted"
			if opts.useTrash {
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
	fmt.Printf("\nCleanup complete. Removed %d items.\n", deletedCount)
	return exitFound
}
