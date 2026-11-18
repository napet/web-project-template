package counter

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"app/backend/db"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var testDB *sql.DB
var testDBURL string

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

// runTests provisions Postgres (testcontainers, or TEST_DATABASE_URL in CI)
// and applies migrations; deferred cleanup runs before os.Exit.
func runTests(m *testing.M) int {
	ctx := context.Background()

	testDBURL = os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		container, err := postgres.Run(ctx,
			"postgres:18",
			postgres.WithDatabase("app_test"),
			postgres.WithUsername("postgres"),
			postgres.WithPassword(randomPassword()),
			postgres.BasicWaitStrategies(),
		)
		if err != nil {
			panic("Could not create database container: " + err.Error())
		}
		defer func() {
			err := testcontainers.TerminateContainer(container)
			if err != nil {
				panic("Could not terminate database container")
			}
		}()

		testDBURL, err = container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			panic("Could not connect to database container")
		}
	}

	var err error
	testDB, err = sql.Open("pgx", testDBURL)
	if err != nil {
		panic("Could not open database")
	}
	defer testDB.Close()
	testDB.SetConnMaxLifetime(5 * time.Minute)

	err = db.MigrateForTests(testDB)
	if err != nil {
		panic("Could not migrate test database: " + err.Error())
	}

	return m.Run()
}

func randomPassword() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic("Could not generate password")
	}
	return hex.EncodeToString(b)
}

func resetCounter(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(`UPDATE counters SET value = 0 WHERE name = $1`, Visitors)
	require.NoError(t, err)
}

func TestMigrationSeedsVisitorsAtZero(t *testing.T) {
	resetCounter(t)
	value, err := NewStore(testDB, Visitors).Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(0), value)
}

func TestIncrementReturnsNewValue(t *testing.T) {
	resetCounter(t)
	store := NewStore(testDB, Visitors)
	ctx := context.Background()

	value, err := store.Increment(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), value)

	value, err = store.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), value)
}

func TestConcurrentIncrementsAreNotLost(t *testing.T) {
	resetCounter(t)
	store := NewStore(testDB, Visitors)
	ctx := context.Background()

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for range n {
		wg.Go(func() {
			_, err := store.Increment(ctx)
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	value, err := store.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(n), value)
}

func TestUnknownCounterFails(t *testing.T) {
	_, err := NewStore(testDB, "missing").Increment(context.Background())
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDatabaseUnavailable(t *testing.T) {
	closed, err := sql.Open("pgx", testDBURL)
	require.NoError(t, err)
	require.NoError(t, closed.Close())

	store := NewStore(closed, Visitors)
	_, err = store.Get(context.Background())
	require.Error(t, err)
	_, err = store.Increment(context.Background())
	require.Error(t, err)
}
