package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidCursor = errors.New("invalid cursor")

type CreatedAtIDCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        int64     `json:"id"`
}

func EncodeCreatedAtIDCursor(cursor CreatedAtIDCursor) (string, error) {
	if cursor.CreatedAt.IsZero() || cursor.ID <= 0 {
		return "", ErrInvalidCursor
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecodeCreatedAtIDCursor(value string) (CreatedAtIDCursor, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return CreatedAtIDCursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return CreatedAtIDCursor{}, ErrInvalidCursor
	}
	var cursor CreatedAtIDCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return CreatedAtIDCursor{}, ErrInvalidCursor
	}
	if cursor.CreatedAt.IsZero() || cursor.ID <= 0 {
		return CreatedAtIDCursor{}, ErrInvalidCursor
	}
	return cursor, nil
}
