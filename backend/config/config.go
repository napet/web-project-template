package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the runtime settings read from the environment.
type Config struct {
	// AllowedOrigins are the browser origins allowed to call the API cross-origin.
	AllowedOrigins []string
	// TrustedProxies are the proxy IPs/CIDRs whose X-Forwarded-For header is trusted.
	TrustedProxies []string
	// Port is the TCP port to listen on.
	Port string
	// LogClientHeaders adds full client addresses and forwarding headers to the
	// request log, to diagnose TRUSTED_PROXIES. Off by default: it logs personal data.
	LogClientHeaders bool
}

var ErrMissingAllowedOrigins = errors.New("ALLOWED_ORIGINS environment variable is required")

const defaultPort = "8080"

// Load reads the configuration from environment variables.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	origins := splitList(getenv("ALLOWED_ORIGINS"))
	for i, origin := range origins {
		origins[i] = strings.TrimRight(origin, "/")
	}
	if len(origins) == 0 {
		return Config{}, ErrMissingAllowedOrigins
	}

	port := getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	logClientHeaders, err := parseBool("LOG_CLIENT_HEADERS", getenv("LOG_CLIENT_HEADERS"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		AllowedOrigins:   origins,
		TrustedProxies:   splitList(getenv("TRUSTED_PROXIES")),
		Port:             port,
		LogClientHeaders: logClientHeaders,
	}, nil
}

// parseBool reads an optional boolean variable; empty means false.
func parseBool(name, value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", name, value)
	}
	return b, nil
}

// splitList splits a comma-separated value, trimming spaces and dropping empty items.
func splitList(value string) []string {
	var items []string
	for item := range strings.SplitSeq(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
