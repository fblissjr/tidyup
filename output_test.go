package main

import (
	"encoding/json"
	"testing"
)

func TestRecordJSONMarshal(t *testing.T) {
	r := Record{
		Type:      "venv",
		Path:      "/home/user/project/.venv",
		Size:      1048576,
		SizeHuman: "1.0 MB",
		LastUsed:  "2025-01-01",
		AgeDays:   90.5,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if m["type"] != "venv" {
		t.Errorf("expected type=venv, got %v", m["type"])
	}
	if m["path"] != "/home/user/project/.venv" {
		t.Errorf("expected correct path, got %v", m["path"])
	}
	if m["size_bytes"].(float64) != 1048576 {
		t.Errorf("expected size_bytes=1048576, got %v", m["size_bytes"])
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
