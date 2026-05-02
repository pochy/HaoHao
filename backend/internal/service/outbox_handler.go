package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	db "example.com/haohao/backend/internal/db"
)

var ErrUnknownOutboxEvent = errors.New("unknown outbox event")

type DefaultOutboxHandler struct {
	emailSender   EmailSender
	notifications *NotificationService
	invitations   *TenantInvitationService
	dataExports   *TenantDataExportService
	webhooks      *WebhookService
	imports       *CustomerSignalImportService
	driveOCR      *DriveOCRService
	datasets      *DatasetService
}

func NewOutboxHandler(emailSender EmailSender, notifications *NotificationService, invitations *TenantInvitationService, dataExports *TenantDataExportService, extras ...any) *DefaultOutboxHandler {
	handler := &DefaultOutboxHandler{
		emailSender:   emailSender,
		notifications: notifications,
		invitations:   invitations,
		dataExports:   dataExports,
	}
	for _, extra := range extras {
		switch item := extra.(type) {
		case *WebhookService:
			handler.webhooks = item
		case *CustomerSignalImportService:
			handler.imports = item
		case *DriveOCRService:
			handler.driveOCR = item
		case *DatasetService:
			handler.datasets = item
		}
	}
	return handler
}

func (h *DefaultOutboxHandler) HandleOutboxEvent(ctx context.Context, event db.OutboxEvent) error {
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
	case "webhook.delivery_requested":
		var payload struct {
			DeliveryID int64 `json:"deliveryId"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if h.webhooks == nil {
			return nil
		}
		return h.webhooks.Deliver(ctx, payload.DeliveryID)
	case "customer_signal_import.requested":
		var payload struct {
			ImportJobID int64 `json:"importJobId"`
			TenantID    int64 `json:"tenantId"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if h.imports == nil {
			return nil
		}
		return h.imports.HandleRequested(ctx, payload.TenantID, payload.ImportJobID)
	case "drive.ocr.requested":
		var payload struct {
			TenantID     int64  `json:"tenantId"`
			FileObjectID int64  `json:"fileObjectId"`
			FilePublicID string `json:"filePublicId"`
			ActorUserID  int64  `json:"actorUserId"`
			Reason       string `json:"reason"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if h.driveOCR == nil {
			return nil
		}
		return h.driveOCR.HandleRequested(ctx, payload.TenantID, payload.FileObjectID, payload.ActorUserID, payload.Reason, event.ID)
	case "dataset.import_requested":
		var payload struct {
			TenantID    int64 `json:"tenantId"`
			ImportJobID int64 `json:"importJobId"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if h.datasets == nil {
			return nil
		}
		return h.datasets.HandleImportRequested(ctx, payload.TenantID, payload.ImportJobID, event.ID)
	case "dataset.work_table_promote_requested":
		var payload struct {
			TenantID    int64 `json:"tenantId"`
			DatasetID   int64 `json:"datasetId"`
			WorkTableID int64 `json:"workTableId"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if h.datasets == nil {
			return nil
		}
		return h.datasets.HandleWorkTablePromotionRequested(ctx, payload.TenantID, payload.DatasetID, payload.WorkTableID)
	case "dataset.work_table_export_requested":
		var payload struct {
			TenantID int64 `json:"tenantId"`
			ExportID int64 `json:"exportId"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if h.datasets == nil {
			return nil
		}
		return h.datasets.HandleWorkTableExportRequested(ctx, payload.TenantID, payload.ExportID)
	case "dataset.sync_requested":
		var payload struct {
			TenantID  int64 `json:"tenantId"`
			SyncJobID int64 `json:"syncJobId"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if h.datasets == nil {
			return nil
		}
		return h.datasets.HandleDatasetSyncRequested(ctx, payload.TenantID, payload.SyncJobID, event.ID)
	default:
		return fmt.Errorf("%w: %s", ErrUnknownOutboxEvent, event.EventType)
	}
}
