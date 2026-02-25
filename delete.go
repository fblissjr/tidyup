package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

// deleteRecords handles the interactive or confirmed deletion of venv records.
func deleteRecords(records []VenvRecord, opts *options) int {
	// Validate --trash on non-macOS.
	if opts.useTrash && runtime.GOOS != "darwin" {
		fmt.Fprintf(os.Stderr, "Warning: -trash is only supported on macOS. Using permanent delete.\n")
		opts.useTrash = false
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

	// Prompt unless --confirm is set.
	if !opts.confirm {
		if opts.useTrash {
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
	fmt.Printf("\nCleanup complete. Removed %d environments.\n", deletedCount)
	return exitFound
}
