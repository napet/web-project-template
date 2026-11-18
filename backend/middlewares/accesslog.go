package middlewares

import (
	"log/slog"
	"net/netip"
	"time"

	"github.com/gin-gonic/gin"
)

// forwardingHeaders are the request headers proxies use to pass on the client
// address. They are logged only in diagnostic mode.
var forwardingHeaders = []string{"X-Forwarded-For", "X-Real-Ip", "Cf-Connecting-Ip", "True-Client-Ip"}

// AccessLog logs one JSON line per request with the method, path, status,
// duration and the client as the rate limits see it, shortened to its /24
// (IPv4) or /48 (IPv6) network so the logs hold no full visitor addresses.
// Health checks are logged at debug level only, to keep Render's probes out of
// the log.
//
// With logClientHeaders the line also carries the direct peer address, the
// full client IP and the forwarding headers. That is personal data; enable it
// only briefly, to find the right TRUSTED_PROXIES setting (see README).
func AccessLog(logger *slog.Logger, logClientHeaders bool, healthPath string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		ctx.Next()

		level := slog.LevelInfo
		if ctx.Request.URL.Path == healthPath {
			level = slog.LevelDebug
		}
		clientIP := ctx.ClientIP()
		attrs := []slog.Attr{
			slog.String("method", ctx.Request.Method),
			slog.String("path", ctx.Request.URL.Path),
			slog.Int("status", ctx.Writer.Status()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("client", ShortenIP(clientIP)),
		}
		if logClientHeaders {
			attrs = append(attrs,
				slog.String("client_ip", clientIP),
				slog.String("remote_addr", ctx.Request.RemoteAddr),
			)
			for _, name := range forwardingHeaders {
				if value := ctx.GetHeader(name); value != "" {
					attrs = append(attrs, slog.String("header_"+name, value))
				}
			}
		}
		logger.LogAttrs(ctx.Request.Context(), level, "request", attrs...)
	}
}

// ShortenIP returns the /24 network of an IPv4 address or the /48 network of
// an IPv6 address, enough to tell clients and proxies apart without logging
// who the visitor is. Values that are not IP addresses become "invalid".
func ShortenIP(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return "invalid"
	}
	addr = addr.Unmap().WithZone("")
	bits := 48
	if addr.Is4() {
		bits = 24
	}
	prefix, err := addr.Prefix(bits)
	if err != nil {
		return "invalid"
	}
	return prefix.String()
}
