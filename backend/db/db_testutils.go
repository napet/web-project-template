package db

import "database/sql"

// MigrateForTests applies all embedded migrations to a test database.
func MigrateForTests(db *sql.DB) error {
	return Migrate(db)
}
