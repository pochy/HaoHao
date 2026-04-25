package service

import (
	"errors"
	"testing"
)

func TestNormalizeCustomerSignalCreateInput(t *testing.T) {
	got, err := normalizeCustomerSignalCreateInput(CustomerSignalCreateInput{
		CustomerName: "  Acme  ",
		Title:        "  Export CSV  ",
		Body:         "  monthly reports  ",
	})
	if err != nil {
		t.Fatalf("normalizeCustomerSignalCreateInput() error = %v", err)
	}
	if got.CustomerName != "Acme" || got.Title != "Export CSV" || got.Body != "monthly reports" {
		t.Fatalf("normalizeCustomerSignalCreateInput() = %#v", got)
	}
	if got.Source != defaultCustomerSignalSource {
		t.Fatalf("Source = %q, want %q", got.Source, defaultCustomerSignalSource)
	}
	if got.Priority != defaultCustomerSignalPriority {
		t.Fatalf("Priority = %q, want %q", got.Priority, defaultCustomerSignalPriority)
	}
	if got.Status != defaultCustomerSignalStatus {
		t.Fatalf("Status = %q, want %q", got.Status, defaultCustomerSignalStatus)
	}

	if _, err := normalizeCustomerSignalCreateInput(CustomerSignalCreateInput{
		CustomerName: "Acme",
		Title:        "Export CSV",
		Source:       "bad-source",
	}); !errors.Is(err, ErrInvalidCustomerSignalInput) {
		t.Fatalf("normalizeCustomerSignalCreateInput(invalid source) error = %v, want %v", err, ErrInvalidCustomerSignalInput)
	}
}

func TestNormalizeCustomerSignalText(t *testing.T) {
	if _, err := normalizeCustomerSignalText("   ", maxCustomerSignalTitleLength, true); !errors.Is(err, ErrInvalidCustomerSignalInput) {
		t.Fatalf("normalizeCustomerSignalText(empty required) error = %v, want %v", err, ErrInvalidCustomerSignalInput)
	}

	longBody := make([]rune, maxCustomerSignalBodyLength+1)
	for i := range longBody {
		longBody[i] = 'a'
	}
	if _, err := normalizeCustomerSignalText(string(longBody), maxCustomerSignalBodyLength, false); !errors.Is(err, ErrInvalidCustomerSignalInput) {
		t.Fatalf("normalizeCustomerSignalText(long body) error = %v, want %v", err, ErrInvalidCustomerSignalInput)
	}
}

func TestParseCustomerSignalPublicIDHidesInvalidUUIDAsNotFound(t *testing.T) {
	if _, err := parseCustomerSignalPublicID("not-a-uuid"); !errors.Is(err, ErrCustomerSignalNotFound) {
		t.Fatalf("parseCustomerSignalPublicID() error = %v, want %v", err, ErrCustomerSignalNotFound)
	}
}
