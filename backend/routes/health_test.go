package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthCheckWithoutDatabase(t *testing.T) {
	router := newRouter(t, Options{Store: &fakeStore{err: errDBDown}})
	rec := do(t, router, request{method: http.MethodGet, path: "/api/v1/health"})

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"message":"Server OK"}`, rec.Body.String())
}
