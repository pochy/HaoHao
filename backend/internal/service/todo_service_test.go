package service

import (
	"errors"
	"testing"
)

func TestNormalizeTodoTitle(t *testing.T) {
	got, err := normalizeTodoTitle("  ship P2  ")
	if err != nil {
		t.Fatalf("normalizeTodoTitle() error = %v", err)
	}
	if got != "ship P2" {
		t.Fatalf("normalizeTodoTitle() = %q, want %q", got, "ship P2")
	}

	if _, err := normalizeTodoTitle("   "); !errors.Is(err, ErrInvalidTodoTitle) {
		t.Fatalf("normalizeTodoTitle(empty) error = %v, want %v", err, ErrInvalidTodoTitle)
	}

	longTitle := make([]rune, maxTodoTitleLength+1)
	for i := range longTitle {
		longTitle[i] = 'a'
	}
	if _, err := normalizeTodoTitle(string(longTitle)); !errors.Is(err, ErrInvalidTodoTitle) {
		t.Fatalf("normalizeTodoTitle(long) error = %v, want %v", err, ErrInvalidTodoTitle)
	}
}

func TestParseTodoPublicIDHidesInvalidUUIDAsNotFound(t *testing.T) {
	if _, err := parseTodoPublicID("not-a-uuid"); !errors.Is(err, ErrTodoNotFound) {
		t.Fatalf("parseTodoPublicID() error = %v, want %v", err, ErrTodoNotFound)
	}
}
