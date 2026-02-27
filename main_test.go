package main

import (
	"testing"
)

func TestParseScanTypes_Default(t *testing.T) {
	types, warnings := parseScanTypes("", false)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if !types["venv"] {
		t.Error("expected venv to be default type")
	}
	if len(types) != 1 {
		t.Errorf("expected 1 type, got %d", len(types))
	}
}

func TestParseScanTypes_All(t *testing.T) {
	types, warnings := parseScanTypes("", true)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	expected := []string{"venv", "node_modules", "pycache", "pytest_cache", "mypy_cache", "ruff_cache", "dist", "build"}
	for _, e := range expected {
		if !types[e] {
			t.Errorf("expected type %q to be set with --all", e)
		}
	}
}

func TestParseScanTypes_Specific(t *testing.T) {
	types, warnings := parseScanTypes("node_modules,pycache", false)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if !types["node_modules"] {
		t.Error("expected node_modules")
	}
	if !types["pycache"] {
		t.Error("expected pycache")
	}
	if types["venv"] {
		t.Error("venv should not be set when --type is used")
	}
}

func TestParseScanTypes_Invalid(t *testing.T) {
	types, warnings := parseScanTypes("venv,typo", false)
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !types["venv"] {
		t.Error("valid type venv should still be set")
	}
	if types["typo"] {
		t.Error("invalid type should not be set")
	}
}

func TestParseScanTypes_WhitespaceHandling(t *testing.T) {
	types, warnings := parseScanTypes(" venv , node_modules ", false)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if !types["venv"] || !types["node_modules"] {
		t.Error("expected both types to be set")
	}
}
