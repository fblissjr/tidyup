package main

import (
	"testing"
)

func TestParseSelection_Single(t *testing.T) {
	got, err := parseSelection("3", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[int]bool{2: true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing index %d", k)
		}
	}
}

func TestParseSelection_Comma(t *testing.T) {
	got, err := parseSelection("1,3,5", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[int]bool{0: true, 2: true, 4: true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing index %d", k)
		}
	}
}

func TestParseSelection_Range(t *testing.T) {
	got, err := parseSelection("1-3", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[int]bool{0: true, 1: true, 2: true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing index %d", k)
		}
	}
}

func TestParseSelection_Mixed(t *testing.T) {
	got, err := parseSelection("1-3,5", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[int]bool{0: true, 1: true, 2: true, 4: true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing index %d", k)
		}
	}
}

func TestParseSelection_All(t *testing.T) {
	got, err := parseSelection("all", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d items, want 5", len(got))
	}
	for i := 0; i < 5; i++ {
		if !got[i] {
			t.Errorf("missing index %d", i)
		}
	}
}

func TestParseSelection_None(t *testing.T) {
	got, err := parseSelection("none", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d items, want 0", len(got))
	}
}

func TestParseSelection_Empty(t *testing.T) {
	got, err := parseSelection("", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d items, want 0", len(got))
	}
}

func TestParseSelection_OutOfRange(t *testing.T) {
	_, err := parseSelection("99", 5)
	if err == nil {
		t.Fatal("expected error for out-of-range selection")
	}
}

func TestParseSelection_InvalidInput(t *testing.T) {
	_, err := parseSelection("abc", 5)
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestParseSelection_ZeroIndex(t *testing.T) {
	_, err := parseSelection("0", 5)
	if err == nil {
		t.Fatal("expected error for zero index (1-based input)")
	}
}

func TestParseSelection_NegativeIndex(t *testing.T) {
	_, err := parseSelection("-1", 5)
	if err == nil {
		t.Fatal("expected error for negative index")
	}
}

func TestParseSelection_ReversedRange(t *testing.T) {
	_, err := parseSelection("3-1", 5)
	if err == nil {
		t.Fatal("expected error for reversed range")
	}
}

func TestParseSelection_WhitespaceHandling(t *testing.T) {
	got, err := parseSelection(" 1 , 3 ", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[int]bool{0: true, 2: true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing index %d", k)
		}
	}
}
