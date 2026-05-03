package service

import "testing"

func TestCanonicalizeValueNormalizesJapaneseDates(t *testing.T) {
	tests := map[string]string{
		"2026年04月28日": "2026-04-28",
		"2026/04/28":  "2026-04-28",
		"２０２６年４月８日":   "2026-04-08",
	}
	for input, want := range tests {
		if got := canonicalizeValue(input, []string{"zenkaku_to_hankaku_basic", "normalize_date"}, nil); got != want {
			t.Fatalf("canonicalizeValue(%q) = %q, want %q", input, got, want)
		}
	}
}
