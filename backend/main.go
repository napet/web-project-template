package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"app/backend/config"
	"app/backend/countcache"
	"app/backend/counter"
	"app/backend/db"
	"app/backend/limiter"
	"app/backend/routes"
	"syscall"
	"time"
)

// countRefresh is how often the cached visitor count is re-read from the
// database, to pick up changes made outside the API.
const countRefresh = time.Minute

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// local development convenience; real environment variables win
	loaded, err := config.LoadDotEnv(".env")
	if err != nil {
		slog.Error("could not read .env", "err", err)
		os.Exit(1)
	}
	if loaded {
		slog.Info("loaded variables from .env")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	if cfg.LogClientHeaders {
		slog.Warn("LOG_CLIENT_HEADERS is on: full client addresses are logged; turn it off once TRUSTED_PROXIES is set")
	}

	err = db.InitDB()
	if err != nil {
		slog.Error("database init failed", "err", err)
		os.Exit(1)
	}

	mainCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	incrementLimiter := limiter.NewWindow(routes.IncrementWindow, nil)
	go incrementLimiter.RunSweeper(mainCtx, routes.IncrementWindow)

	handler, err := routes.NewRouter(routes.Options{
		AllowedOrigins: cfg.AllowedOrigins,
		TrustedProxies: cfg.TrustedProxies,
		// reads come from memory; only increments and a periodic refresh reach the database
		Store:            countcache.New(counter.NewStore(db.DB, counter.Visitors), countRefresh, nil),
		IncrementLimiter: incrementLimiter,
		LogClientHeaders: cfg.LogClientHeaders,
	})
	if err != nil {
		slog.Error("router setup failed", "err", err)
		os.Exit(1)
	}

	server := newServer(":"+cfg.Port, handler, serverTimeouts)

	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()
	slog.Info("listening", "port", cfg.Port)

	<-mainCtx.Done()
	slog.Info("server shutting down")

	// open event streams only end when their request context is cancelled
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = server.Shutdown(shutdownCtx)
	if err != nil {
		slog.Error("server shutdown failed", "err", err)
	}
}

// timeouts bound how long a client may hold a connection without making progress.
type timeouts struct {
	readHeader, read, idle time.Duration
}

var serverTimeouts = timeouts{readHeader: 5 * time.Second, read: 10 * time.Second, idle: 60 * time.Second}

// newServer configures the HTTP server. Requests carry no body, so reading one
// never needs long, and idle keep-alive connections are closed so they cannot
// pile up. There is no WriteTimeout because event streams stay open for as long
// as the client is connected; the stream caps in routes bound those instead.
func newServer(addr string, handler http.Handler, t timeouts) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: t.readHeader,
		ReadTimeout:       t.read,
		IdleTimeout:       t.idle,
		MaxHeaderBytes:    16 << 10,
	}
}
