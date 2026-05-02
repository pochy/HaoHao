package service

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeSavedFilterListInput(t *testing.T) {
	got, cursor, err := normalizeSavedFilterListInput(CustomerSignalSavedFilterListInput{
		Query:    "  weekly export  ",
		Status:   " Planned ",
		Priority: " HIGH ",
		Source:   " Sales ",
		Limit:    500,
	})
	if err != nil {
		t.Fatalf("normalizeSavedFilterListInput() error = %v", err)
	}
	if !cursor.CreatedAt.IsZero() || cursor.ID != 0 {
		t.Fatalf("cursor = %#v, want empty cursor", cursor)
	}
	if got.Query != "weekly export" || got.Status != "planned" || got.Priority != "high" || got.Source != "sales" {
		t.Fatalf("normalizeSavedFilterListInput() = %#v", got)
	}
	if got.Limit != maxCustomerSignalSavedFilterListLimit {
		t.Fatalf("Limit = %d, want %d", got.Limit, maxCustomerSignalSavedFilterListLimit)
	}

	got, _, err = normalizeSavedFilterListInput(CustomerSignalSavedFilterListInput{})
	if err != nil {
		t.Fatalf("normalizeSavedFilterListInput(defaults) error = %v", err)
	}
	if got.Limit != defaultCustomerSignalSavedFilterListLimit {
		t.Fatalf("default Limit = %d, want %d", got.Limit, defaultCustomerSignalSavedFilterListLimit)
	}
}

func TestNormalizeSavedFilterListInputRejectsInvalidValues(t *testing.T) {
	longQuery := strings.Repeat("a", maxCustomerSignalSavedFilterSearchLength+1)
	cases := []CustomerSignalSavedFilterListInput{
		{Query: longQuery},
		{Status: "bad"},
		{Priority: "bad"},
		{Source: "bad"},
		{Cursor: "bad"},
	}
	for _, tc := range cases {
		if _, _, err := normalizeSavedFilterListInput(tc); !errors.Is(err, ErrInvalidCustomerSignalSavedFilter) && !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("normalizeSavedFilterListInput(%#v) error = %v, want invalid saved filter or cursor", tc, err)
		}
	}
}
