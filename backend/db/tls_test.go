package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLSWarning(t *testing.T) {
	const remote = "postgres://u:p@ep-example.eu-central-1.aws.neon.tech/neondb"
	cases := map[string]bool{ // connection string → expect a warning
		remote + "?sslmode=verify-full":                           false,
		remote + "?sslmode=verify-full&channel_binding=require":   false,
		remote + "?sslmode=require":                               true,
		remote + "?sslmode=prefer":                                true,
		remote + "?sslmode=disable":                               true,
		remote:                                                    true, // defaults to prefer
		"postgres://u:p@localhost:5432/app?sslmode=disable": false,
		"postgres://u:p@127.0.0.1:5432/app?sslmode=disable": false,
	}
	for conn, wantWarning := range cases {
		warning, err := tlsWarning(conn)
		require.NoError(t, err, conn)
		require.Equal(t, wantWarning, warning != "", "%s: %q", conn, warning)
	}
}
