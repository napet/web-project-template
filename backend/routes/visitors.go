package routes

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"app/backend/events"
	"app/backend/limiter"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	visitorsTopic = "visitors"
	countEvent    = "count"
	// sseRetryMillis tells EventSource clients how long to wait before reconnecting.
	sseRetryMillis = 5000
	// streamsFullRetrySeconds is the Retry-After sent when the global stream cap is reached.
	streamsFullRetrySeconds = 30
)

type visitorsHandler struct {
	store     CounterStore
	hub       *events.Hub
	limiter   *limiter.Window
	streams   *limiter.ConnCap
	heartbeat time.Duration
}

func unavailable(ctx *gin.Context, err error) {
	slog.ErrorContext(ctx.Request.Context(), "counter unavailable", "err", err)
	ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"message": "Counter temporarily unavailable"})
}

// get returns the current visitor count.
func (h *visitorsHandler) get(ctx *gin.Context) {
	count, err := h.store.Get(ctx.Request.Context())
	if err != nil {
		unavailable(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"count": count})
}

// increment adds one visit, at most once per client per window, and
// broadcasts the committed value to stream subscribers.
func (h *visitorsHandler) increment(ctx *gin.Context) {
	client := limiter.ClientKey(ctx.ClientIP())
	allowed, retryAfter := h.limiter.Allow(client)
	if !allowed {
		seconds := max(1, int(math.Ceil(retryAfter.Seconds())))
		ctx.Header("Retry-After", strconv.Itoa(seconds))
		ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": "Too many requests"})
		return
	}

	count, err := h.store.Increment(ctx.Request.Context())
	if err != nil {
		// the visit was not counted, so do not hold the client's window
		h.limiter.Release(client)
		unavailable(ctx, err)
		return
	}

	h.hub.Publish(visitorsTopic, events.Event{Type: countEvent, Data: count})
	ctx.JSON(http.StatusOK, gin.H{"count": count})
}

// stream sends the current count on connect, then every change, as
// Server-Sent Events, with periodic heartbeat comments.
func (h *visitorsHandler) stream(ctx *gin.Context) {
	client := limiter.ClientKey(ctx.ClientIP())
	switch h.streams.Acquire(client) {
	case limiter.KeyFull:
		ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": "Too many open streams"})
		return
	case limiter.TotalFull:
		slog.WarnContext(ctx.Request.Context(), "stream capacity reached", "max", h.streams.Total())
		ctx.Header("Retry-After", strconv.Itoa(streamsFullRetrySeconds))
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"message": "Too many open streams"})
		return
	}
	defer h.streams.Release(client)

	// subscribe before reading so no increment between the two is missed
	updates, unsubscribe := h.hub.Subscribe(visitorsTopic)
	defer unsubscribe()

	reqCtx := ctx.Request.Context()
	count, err := h.store.Get(reqCtx)
	if err != nil {
		unavailable(ctx, err)
		return
	}

	w := ctx.Writer
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, "retry: %d\n\n", sseRetryMillis)
	writeCount(w, count)
	w.Flush()

	heartbeat := time.NewTicker(h.heartbeat)
	defer heartbeat.Stop()

	for {
		select {
		case <-reqCtx.Done():
			return
		case event, ok := <-updates:
			if !ok {
				return
			}
			writeCount(w, event.Data)
			w.Flush()
		case <-heartbeat.C:
			_, _ = io.WriteString(w, ": ping\n\n")
			w.Flush()
		}
	}
}

func writeCount(w io.Writer, count any) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %v\n\n", countEvent, count)
}
