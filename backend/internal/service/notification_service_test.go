package service

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeNotificationListInput(t *testing.T) {
	got, cursor, err := normalizeNotificationListInput(NotificationListInput{
		Query:     "  Invite  ",
		ReadState: " UNREAD ",
		Channel:   " IN_APP ",
		Limit:     250,
	})
	if err != nil {
		t.Fatalf("normalizeNotificationListInput() error = %v", err)
	}
	if cursor.ID != 0 {
		t.Fatalf("cursor = %#v, want zero cursor", cursor)
	}
	if got.Query != "Invite" || got.ReadState != notificationReadStateUnread || got.Channel != "in_app" {
		t.Fatalf("normalizeNotificationListInput() = %#v", got)
	}
	if got.Limit != maxNotificationListLimit {
		t.Fatalf("Limit = %d, want %d", got.Limit, maxNotificationListLimit)
	}

	got, _, err = normalizeNotificationListInput(NotificationListInput{})
	if err != nil {
		t.Fatalf("normalizeNotificationListInput(defaults) error = %v", err)
	}
	if got.ReadState != notificationReadStateAll || got.Limit != defaultNotificationListLimit {
		t.Fatalf("normalizeNotificationListInput(defaults) = %#v", got)
	}
}

func TestNormalizeNotificationListInputRejectsInvalidValues(t *testing.T) {
	cases := []NotificationListInput{
		{ReadState: "archived"},
		{Channel: "sms"},
		{Query: strings.Repeat("a", maxNotificationSearchLength+1)},
		{Cursor: "bad-cursor"},
	}
	for _, tc := range cases {
		if _, _, err := normalizeNotificationListInput(tc); !errors.Is(err, ErrInvalidNotification) && !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("normalizeNotificationListInput(%#v) error = %v, want invalid notification or cursor", tc, err)
		}
	}
}

func TestParseNotificationPublicIDs(t *testing.T) {
	got, err := parseNotificationPublicIDs([]string{
		"018f2f05-c6c9-7a49-b32d-04f4dd84ef4a",
		" 018f2f05-c6c9-7a49-b32d-04f4dd84ef4a ",
		"018f2f05-c6c9-7a49-b32d-04f4dd84ef4b",
	})
	if err != nil {
		t.Fatalf("parseNotificationPublicIDs() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(parseNotificationPublicIDs()) = %d, want 2", len(got))
	}

	if _, err := parseNotificationPublicIDs([]string{"not-a-uuid"}); !errors.Is(err, ErrInvalidNotification) {
		t.Fatalf("parseNotificationPublicIDs(invalid) error = %v, want %v", err, ErrInvalidNotification)
	}
}
