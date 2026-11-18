package routes

import (
	"context"
	"log/slog"
	"app/backend/events"
	"app/backend/limiter"
	"app/backend/middlewares"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// healthPath is the health check route; it is kept out of the info-level request log.
const healthPath = "/api/v1/health"

const (
	// DefaultHeartbeat keeps idle streams below typical 30 s proxy idle timeouts.
	DefaultHeartbeat = 25 * time.Second
	// IncrementWindow is how often one client IP may increment the counter.
	IncrementWindow = 10 * time.Minute
	// MaxStreamsPerIP caps concurrent stream connections per client (an IPv4
	// address or an IPv6 /64, see limiter.ClientKey).
	MaxStreamsPerIP = 5
	// MaxStreams caps concurrent stream connections across all clients, so many
	// clients together cannot exhaust the process's memory or file descriptors.
	MaxStreams = 300
)

// CounterStore reads and increments the visitor counter.
type CounterStore interface {
	Get(ctx context.Context) (int64, error)
	Increment(ctx context.Context) (int64, error)
}

// Options are the dependencies and settings of the HTTP router.
type Options struct {
	AllowedOrigins []string
	// TrustedProxies whose X-Forwarded-For is honoured; empty trusts none.
	TrustedProxies []string
	Store          CounterStore
	// Optional; defaults are created when nil or zero.
	Hub              *events.Hub
	IncrementLimiter *limiter.Window
	StreamCap        *limiter.ConnCap
	Heartbeat        time.Duration
	// LogClientHeaders adds full client addresses and forwarding headers to the request log.
	LogClientHeaders bool
}

// NewRouter builds the Gin engine with middleware and all API routes.
func NewRouter(opts Options) (*gin.Engine, error) {
	if opts.Hub == nil {
		opts.Hub = events.NewHub()
	}
	if opts.IncrementLimiter == nil {
		opts.IncrementLimiter = limiter.NewWindow(IncrementWindow, nil)
	}
	if opts.StreamCap == nil {
		opts.StreamCap = limiter.NewConnCap(MaxStreamsPerIP, MaxStreams)
	}
	if opts.Heartbeat == 0 {
		opts.Heartbeat = DefaultHeartbeat
	}

	router := gin.New()
	if gin.Mode() != gin.TestMode {
		router.Use(middlewares.AccessLog(slog.Default(), opts.LogClientHeaders, healthPath))
	}
	router.Use(gin.Recovery())

	// gin trusts every proxy by default; restrict it so ClientIP cannot be spoofed.
	err := router.SetTrustedProxies(opts.TrustedProxies)
	if err != nil {
		return nil, err
	}

	router.Use(middlewares.CORS(opts.AllowedOrigins))

	visitors := &visitorsHandler{
		store:     opts.Store,
		hub:       opts.Hub,
		limiter:   opts.IncrementLimiter,
		streams:   opts.StreamCap,
		heartbeat: opts.Heartbeat,
	}

	v1 := router.Group("/api/v1")
	v1.GET(strings.TrimPrefix(healthPath, "/api/v1"), healthCheck)
	v1.GET("/visitors", visitors.get)
	v1.POST("/visitors", visitors.increment)
	v1.GET("/visitors/stream", visitors.stream)

	return router, nil
}
