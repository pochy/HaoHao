package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidWebhookSignature = errors.New("invalid webhook signature")

func BuildWebhookSignature(secret string, timestamp time.Time, body []byte) string {
	unix := timestamp.Unix()
	base := fmt.Sprintf("%d.", unix)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(base))
	mac.Write(body)
	return fmt.Sprintf("t=%d,v1=%s", unix, hex.EncodeToString(mac.Sum(nil)))
}

func VerifyWebhookSignature(secret string, now time.Time, tolerance time.Duration, body []byte, header string) error {
	var ts int64
	var signature string
	for _, part := range strings.Split(header, ",") {
		keyValue := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(keyValue) != 2 {
			continue
		}
		switch keyValue[0] {
		case "t":
			parsed, err := strconv.ParseInt(keyValue[1], 10, 64)
			if err != nil {
				return ErrInvalidWebhookSignature
			}
			ts = parsed
		case "v1":
			signature = keyValue[1]
		}
	}
	if ts <= 0 || signature == "" {
		return ErrInvalidWebhookSignature
	}
	signedAt := time.Unix(ts, 0)
	if tolerance > 0 && (now.Sub(signedAt) > tolerance || signedAt.Sub(now) > tolerance) {
		return ErrInvalidWebhookSignature
	}
	expected := BuildWebhookSignature(secret, signedAt, body)
	expectedParts := strings.Split(expected, "v1=")
	if len(expectedParts) != 2 {
		return ErrInvalidWebhookSignature
	}
	if hmac.Equal([]byte(signature), []byte(expectedParts[1])) {
		return nil
	}
	return ErrInvalidWebhookSignature
}
