package counter

import (
	"context"
	"database/sql"
	"fmt"
)

// Visitors is the name of the site's visitor counter row.
const Visitors = "visitors"

// Store reads and increments one named counter in the counters table.
type Store struct {
	db   *sql.DB
	name string
}

func NewStore(db *sql.DB, name string) *Store {
	return &Store{db: db, name: name}
}

// Get returns the current value of the counter.
func (s *Store) Get(ctx context.Context) (int64, error) {
	var value int64
	err := s.db.QueryRowContext(ctx, `SELECT value FROM counters WHERE name = $1`, s.name).Scan(&value)
	if err != nil {
		return 0, fmt.Errorf("read counter %q: %w", s.name, err)
	}
	return value, nil
}

// Increment atomically adds one to the counter and returns the new value.
// The row lock taken by UPDATE serialises concurrent increments.
func (s *Store) Increment(ctx context.Context) (int64, error) {
	var value int64
	err := s.db.QueryRowContext(ctx,
		`UPDATE counters SET value = value + 1 WHERE name = $1 RETURNING value`, s.name,
	).Scan(&value)
	if err != nil {
		return 0, fmt.Errorf("increment counter %q: %w", s.name, err)
	}
	return value, nil
}
