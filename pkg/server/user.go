package server

import (
	"fmt"
	"os"
	"path/filepath"
)

func CreateUser(config Config, name string) error {
	if _, err := config.database.conn.Exec("INSERT INTO users (name) VALUES (?)", name); err != nil {
		return fmt.Errorf("CreateUser insert in db: %w", err)
	}

	userDir := filepath.Join(config.ProjectDir, name)
	if err := os.Mkdir(userDir, 0700); err != nil {
		return fmt.Errorf("CreateUser make user dir: %w", err)
	}

	return nil
}

func DeleteUser(config Config, name string) error {
	if _, err := config.database.conn.Exec("DELETE FROM users WHERE name = ?", name); err != nil {
		return fmt.Errorf("DeleteUser delete from db: %w", err)
	}

	userDir := filepath.Join(config.ProjectDir, name)
	if err := os.RemoveAll(userDir); err != nil {
		return fmt.Errorf("DeleteUser RemoveAll dir: %w", err)
	}

	return nil
}
