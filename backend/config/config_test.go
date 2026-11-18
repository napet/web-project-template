package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadRequiresAllowedOrigins(t *testing.T) {
	_, err := load(env(map[string]string{}))
	require.ErrorIs(t, err, ErrMissingAllowedOrigins)
	require.Contains(t, err.Error(), "ALLOWED_ORIGINS")

	_, err = load(env(map[string]string{"ALLOWED_ORIGINS": " , "}))
	require.ErrorIs(t, err, ErrMissingAllowedOrigins)
}

func TestLoadParsesLists(t *testing.T) {
	cfg, err := load(env(map[string]string{
		"ALLOWED_ORIGINS": "https://example.com/, http://localhost:5173",
		"TRUSTED_PROXIES": "10.0.0.0/8",
	}))
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com", "http://localhost:5173"}, cfg.AllowedOrigins)
	require.Equal(t, []string{"10.0.0.0/8"}, cfg.TrustedProxies)
	require.Equal(t, "8080", cfg.Port)
}

func TestLoadUsesPort(t *testing.T) {
	cfg, err := load(env(map[string]string{"ALLOWED_ORIGINS": "https://example.com", "PORT": "10000"}))
	require.NoError(t, err)
	require.Equal(t, "10000", cfg.Port)
	require.Empty(t, cfg.TrustedProxies)
}

func TestLoadClientHeaderLogging(t *testing.T) {
	base := map[string]string{"ALLOWED_ORIGINS": "https://example.com"}

	cfg, err := load(env(base))
	require.NoError(t, err)
	require.False(t, cfg.LogClientHeaders, "off by default")

	base["LOG_CLIENT_HEADERS"] = "true"
	cfg, err = load(env(base))
	require.NoError(t, err)
	require.True(t, cfg.LogClientHeaders)

	base["LOG_CLIENT_HEADERS"] = "sometimes"
	_, err = load(env(base))
	require.ErrorContains(t, err, "LOG_CLIENT_HEADERS")
}
