package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func CreateUser(config Config, name string) error {
	if _, err := config.Database.conn.Exec("INSERT INTO users (name) VALUES (?)", name); err != nil {
		return fmt.Errorf("CreateUser insert in db: %w", err)
	}

	userDir := filepath.Join(config.ProjectDir, name)
	if err := os.Mkdir(userDir, 0700); err != nil {
		return fmt.Errorf("CreateUser make user dir: %w", err)
	}

	return nil
}
