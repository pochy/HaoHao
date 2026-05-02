package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type RealtimeEventBody struct {
	Cursor           string         `json:"cursor"`
	PublicID         string         `json:"publicId"`
	TenantID         *int64         `json:"tenantId,omitempty"`
	Type             string         `json:"type"`
	ResourceType     string         `json:"resourceType,omitempty"`
	ResourcePublicID string         `json:"resourcePublicId,omitempty"`
	Payload          map[string]any `json:"payload"`
	CreatedAt        time.Time      `json:"createdAt"`
}

type RealtimePollBody struct {
	Items  []RealtimeEventBody `json:"items"`
	Cursor string              `json:"cursor"`
}

func RegisterRawRealtimeRoutes(router *gin.Engine, deps Dependencies) {
	if router == nil {
		return
	}
	router.GET("/api/v1/realtime/events", func(c *gin.Context) {
		current, ok := rawRealtimeSession(c, deps)
		if !ok {
			return
		}
		cursorID, hasCursor, ok := rawRealtimeCursor(c)
		if !ok {
			return
		}
		tenantID := current.ActiveTenantID
		if !hasCursor {
			currentCursor, err := deps.RealtimeService.CurrentCursor(c.Request.Context(), current.User.ID, tenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"title": "realtime cursor is unavailable"})
				return
			}
			cursorID = currentCursor
		}

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"title": "streaming is not supported"})
			return
		}
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache, no-transform")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
		deps.RealtimeService.IncConnection()
		defer deps.RealtimeService.DecConnection()

		if !hasCursor {
			writeSSE(c.Writer, "realtime.ready", service.RealtimeCursor(cursorID), map[string]string{"cursor": service.RealtimeCursor(cursorID)})
			flusher.Flush()
		}

		pubsub := deps.RealtimeService.Subscribe(c.Request.Context(), current.User.ID)
		var redisMessages <-chan *redis.Message
		if pubsub != nil {
			defer pubsub.Close()
			redisMessages = pubsub.Channel()
		}
		heartbeat := time.NewTicker(deps.RealtimeService.HeartbeatInterval())
		defer heartbeat.Stop()
		catchup := time.NewTicker(5 * time.Second)
		defer catchup.Stop()

		sendAvailable := func() bool {
			items, err := deps.RealtimeService.ListAfter(c.Request.Context(), current.User.ID, tenantID, cursorID, deps.RealtimeService.BackfillLimit())
			if err != nil {
				_, _ = fmt.Fprintf(c.Writer, "event: realtime.error\ndata: {\"title\":\"realtime events unavailable\"}\n\n")
				flusher.Flush()
				return false
			}
			for _, item := range items {
				body := toRealtimeEventBody(item)
				writeSSE(c.Writer, body.Type, body.Cursor, body)
				cursorID = item.ID
				deps.RealtimeService.IncDelivered(item.EventType, "sse")
			}
			if len(items) > 0 {
				flusher.Flush()
			}
			return true
		}

		sendAvailable()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case _, ok := <-redisMessages:
				if !ok {
					redisMessages = nil
					continue
				}
				sendAvailable()
			case <-catchup.C:
				sendAvailable()
			case <-heartbeat.C:
				_, _ = c.Writer.Write([]byte(": heartbeat\n\n"))
				flusher.Flush()
			}
		}
	})

	router.GET("/api/v1/realtime/events/poll", func(c *gin.Context) {
		current, ok := rawRealtimeSession(c, deps)
		if !ok {
			return
		}
		cursorID, hasCursor, ok := rawRealtimeCursor(c)
		if !ok {
			return
		}
		tenantID := current.ActiveTenantID
		if !hasCursor {
			currentCursor, err := deps.RealtimeService.CurrentCursor(c.Request.Context(), current.User.ID, tenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"title": "realtime cursor is unavailable"})
				return
			}
			c.JSON(http.StatusOK, RealtimePollBody{Items: []RealtimeEventBody{}, Cursor: service.RealtimeCursor(currentCursor)})
			return
		}

		timeout := realtimePollTimeout(c.Query("timeoutSeconds"), deps.RealtimeService.LongPollTimeout())
		deadline := time.NewTimer(timeout)
		defer deadline.Stop()
		catchup := time.NewTicker(2 * time.Second)
		defer catchup.Stop()
		pubsub := deps.RealtimeService.Subscribe(c.Request.Context(), current.User.ID)
		var redisMessages <-chan *redis.Message
		if pubsub != nil {
			defer pubsub.Close()
			redisMessages = pubsub.Channel()
		}

		sendIfAvailable := func() bool {
			items, err := deps.RealtimeService.ListAfter(c.Request.Context(), current.User.ID, tenantID, cursorID, deps.RealtimeService.BackfillLimit())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"title": "realtime events unavailable"})
				return true
			}
			if len(items) == 0 {
				return false
			}
			bodies := make([]RealtimeEventBody, 0, len(items))
			for _, item := range items {
				bodies = append(bodies, toRealtimeEventBody(item))
				cursorID = item.ID
				deps.RealtimeService.IncDelivered(item.EventType, "poll")
			}
			c.JSON(http.StatusOK, RealtimePollBody{Items: bodies, Cursor: service.RealtimeCursor(cursorID)})
			return true
		}

		if sendIfAvailable() {
			return
		}
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case _, ok := <-redisMessages:
				if !ok {
					redisMessages = nil
					continue
				}
				if sendIfAvailable() {
					return
				}
			case <-catchup.C:
				if sendIfAvailable() {
					return
				}
			case <-deadline.C:
				deps.RealtimeService.IncPollTimeout()
				c.Status(http.StatusNoContent)
				return
			}
		}
	})
}

func rawRealtimeSession(c *gin.Context, deps Dependencies) (service.CurrentSession, bool) {
	if deps.RealtimeService == nil || !deps.RealtimeService.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"title": "realtime service is disabled"})
		return service.CurrentSession{}, false
	}
	if deps.SessionService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"title": "session service is not configured"})
		return service.CurrentSession{}, false
	}
	cookie, err := c.Cookie(auth.SessionCookieName)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"title": "missing or expired session"})
		return service.CurrentSession{}, false
	}
	current, err := deps.SessionService.CurrentSession(c.Request.Context(), cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"title": "missing or expired session"})
		return service.CurrentSession{}, false
	}
	return current, true
}

func rawRealtimeCursor(c *gin.Context) (int64, bool, bool) {
	value := strings.TrimSpace(c.Query("cursor"))
	if value == "" {
		value = strings.TrimSpace(c.GetHeader("Last-Event-ID"))
	}
	cursor, err := service.ParseRealtimeCursor(value)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"title": "invalid realtime cursor"})
		return 0, false, false
	}
	return cursor, value != "", true
}

func writeSSE(w http.ResponseWriter, eventType, cursor string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		payload = []byte(`{"title":"invalid realtime payload"}`)
	}
	if cursor != "" {
		_, _ = fmt.Fprintf(w, "id: %s\n", cursor)
	}
	if eventType != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", eventType)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
}

func toRealtimeEventBody(item service.RealtimeEvent) RealtimeEventBody {
	return RealtimeEventBody{
		Cursor:           item.Cursor,
		PublicID:         item.PublicID,
		TenantID:         item.TenantID,
		Type:             item.EventType,
		ResourceType:     item.ResourceType,
		ResourcePublicID: item.ResourcePublicID,
		Payload:          item.Payload,
		CreatedAt:        item.CreatedAt,
	}
}

func realtimePollTimeout(value string, fallback time.Duration) time.Duration {
	if fallback <= 0 {
		fallback = 25 * time.Second
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		seconds = int(fallback.Seconds())
	}
	if seconds < 1 {
		seconds = 1
	}
	if seconds > 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}
