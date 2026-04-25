package service

import (
	"context"
	"log/slog"
	"strings"
)

type EmailMessage struct {
	ToUserID int64
	Subject  string
	Body     string
}

type EmailSender interface {
	SendEmail(context.Context, EmailMessage) error
}

type LogEmailSender struct {
	logger *slog.Logger
	from   string
}

func NewLogEmailSender(logger *slog.Logger, from string) *LogEmailSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &LogEmailSender{
		logger: logger,
		from:   strings.TrimSpace(from),
	}
}

func (s *LogEmailSender) SendEmail(ctx context.Context, message EmailMessage) error {
	if s == nil {
		return nil
	}
	s.logger.InfoContext(ctx, "email delivery logged",
		"from", s.from,
		"to_user_id", message.ToUserID,
		"subject", message.Subject,
	)
	return nil
}
