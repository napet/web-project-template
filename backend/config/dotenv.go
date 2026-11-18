package config

import (
	"errors"
	"io/fs"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads variables from a local env file for development.
// A missing file is not an error, and variables already set in the real
// environment always take precedence over the file.
func LoadDotEnv(path string) (bool, error) {
	err := godotenv.Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
