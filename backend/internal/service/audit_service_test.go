package service

import (
	"errors"
	"testing"
)

func TestNormalizeAuditEventRequiresActorUser(t *testing.T) {
	_, err := normalizeAuditEvent(AuditEventInput{
		AuditContext: AuditContext{ActorType: AuditActorUser},
		Action:       "todo.create",
		TargetType:   "todo",
		TargetID:     "018f2f05-c6c9-7a49-b32d-04f4dd84ef4a",
	})
	if !errors.Is(err, ErrInvalidAuditEvent) {
		t.Fatalf("normalizeAuditEvent() error = %v, want %v", err, ErrInvalidAuditEvent)
	}
}

func TestNormalizeAuditEventDefaultsMetadata(t *testing.T) {
	userID := int64(1)
	got, err := normalizeAuditEvent(AuditEventInput{
		AuditContext: AuditContext{
			ActorUserID: &userID,
		},
		Action:     " TODO.Create ",
		TargetType: " TODO ",
		TargetID:   " target ",
	})
	if err != nil {
		t.Fatalf("normalizeAuditEvent() error = %v", err)
	}
	if got.Action != "todo.create" {
		t.Fatalf("Action = %q, want %q", got.Action, "todo.create")
	}
	if got.TargetType != "todo" {
		t.Fatalf("TargetType = %q, want %q", got.TargetType, "todo")
	}
	if got.TargetID != "target" {
		t.Fatalf("TargetID = %q, want %q", got.TargetID, "target")
	}
	if got.Metadata == nil {
		t.Fatal("Metadata = nil, want empty map")
	}
}
