package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

const RequestIDHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}

		c.Set("request_id", requestID)
		c.Header(RequestIDHeader, requestID)
		c.Next()
	}
}

func RequestIDFromContext(c *gin.Context) string {
	value, ok := c.Get("request_id")
	if !ok {
		return ""
	}

	requestID, _ := value.(string)
	return requestID
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}

	return hex.EncodeToString(b[:])
}
