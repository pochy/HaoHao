package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	db "example.com/haohao/backend/internal/db"
)

var ErrUnknownOutboxEvent = errors.New("unknown outbox event")

type P7OutboxHandler struct {
	emailSender   EmailSender
	notifications *NotificationService
	invitations   *TenantInvitationService
	dataExports   *TenantDataExportService
}

func NewOutboxHandler(emailSender EmailSender, notifications *NotificationService, invitations *TenantInvitationService, dataExports *TenantDataExportService) *P7OutboxHandler {
	return &P7OutboxHandler{
		emailSender:   emailSender,
		notifications: notifications,
		invitations:   invitations,
		dataExports:   dataExports,
	}
}

func (h *P7OutboxHandler) HandleOutboxEvent(ctx context.Context, event db.OutboxEvent) error {
	if h == nil {
		return nil
	}
	switch event.EventType {
	case "notification.email_requested":
		var payload struct {
			RecipientUserID int64  `json:"recipientUserId"`
			Subject         string `json:"subject"`
			Body            string `json:"body"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if h.emailSender == nil {
			return nil
		}
		return h.emailSender.SendEmail(ctx, EmailMessage{
			ToUserID: payload.RecipientUserID,
			Subject:  payload.Subject,
			Body:     payload.Body,
		})
	case "tenant_invitation.created":
		var payload struct {
			InvitationID int64 `json:"invitationId"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if h.invitations == nil {
			return nil
		}
		return h.invitations.HandleInvitationCreated(ctx, payload.InvitationID)
	case "tenant_data_export.requested":
		var payload struct {
			ExportID int64 `json:"exportId"`
			TenantID int64 `json:"tenantId"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if h.dataExports == nil {
			return nil
		}
		return h.dataExports.HandleRequested(ctx, payload.TenantID, payload.ExportID)
	default:
		return fmt.Errorf("%w: %s", ErrUnknownOutboxEvent, event.EventType)
	}
}
