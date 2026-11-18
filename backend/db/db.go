package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

var DB *sql.DB

//go:embed migrations/*.sql
var embedMigrations embed.FS

// ErrMissingDatabaseURL is returned when DATABASE_URL is not set.
var ErrMissingDatabaseURL = errors.New("DATABASE_URL environment variable is required")

// InitDB opens the connection pool from DATABASE_URL, verifies it and applies migrations.
func InitDB() error {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		return ErrMissingDatabaseURL
	}

	warning, err := tlsWarning(dbUrl)
	if err != nil {
		return err
	}
	if warning != "" {
		slog.Warn(warning)
	}

	db, err := sql.Open("pgx", dbUrl)
	if err != nil {
		return fmt.Errorf("could not open db: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	err = db.Ping()
	if err != nil {
		return fmt.Errorf("could not ping db: %w", err)
	}

	err = Migrate(db)
	if err != nil {
		return fmt.Errorf("could not migrate db: %w", err)
	}

	DB = db
	return nil
}

// Migrate applies all embedded migrations to the given database.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	err := goose.SetDialect("postgres")
	if err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}
