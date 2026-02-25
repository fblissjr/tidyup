package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// VenvRecord holds metadata about a found environment for evaluation.
type VenvRecord struct {
	Path      string  `json:"path"`
	Size      int64   `json:"size_bytes"`
	SizeHuman string  `json:"size_human"`
	LastUsed  string  `json:"last_used"`
	AgeDays   float64 `json:"age_days"`
}

// JSONOutput is the top-level structure for --json output.
type JSONOutput struct {
	Count      int          `json:"count"`
	TotalBytes int64        `json:"total_bytes"`
	TotalHuman string       `json:"total_human"`
	Records    []VenvRecord `json:"records"`
	DryRun     bool         `json:"dry_run"`
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

// sortRecords sorts records by the given field.
func sortRecords(records []VenvRecord, field string) {
	switch field {
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
}

// totalSize sums the size of all records.
func totalSize(records []VenvRecord) int64 {
	var total int64
	for _, r := range records {
		total += r.Size
	}
	return total
}

// printJSON writes machine-readable JSON output.
func printJSON(records []VenvRecord, total int64, dryRun bool) int {
	out := JSONOutput{
		Count:      len(records),
		TotalBytes: total,
		TotalHuman: formatBytes(total),
		Records:    records,
		DryRun:     dryRun,
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

// printText writes human-readable text output.
func printText(records []VenvRecord, total int64) {
	for _, r := range records {
		fmt.Printf("%-10s %-4.0fd ago  %s\n", r.SizeHuman, r.AgeDays, r.Path)
	}
	if len(records) > 0 {
		fmt.Printf("\nFound %d environments totaling %s\n", len(records), formatBytes(total))
	} else {
		fmt.Println("No unused environments found.")
	}
}
