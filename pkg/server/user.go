package server

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const BearerTokenByteLength = 32

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

func CreateUserToken(config Config, userName, tokenDescription string) (string, error) {
	userId, err := config.database.GetUserId(userName)
	if err != nil {
		return "", fmt.Errorf("CreateUserToken get user id: %w", err)
	}

	var buffer [BearerTokenByteLength]byte
	writer := bytes.NewBuffer(buffer[:])
	if _, err := io.Copy(writer, rand.Reader); err != nil {
		return "", fmt.Errorf("CreateUserToken read rand: %w", err)
	}

	token := fmt.Sprintf("%x", buffer)

	if _, err := config.database.conn.Exec(
		"INSERT INTO tokens (user_id, token, description) VALUES (?, ?, ?)",
		userId,
		token,
		tokenDescription,
	); err != nil {
		return "", fmt.Errorf("CreateUserToken insert db: %w", err)
	}

	return token, nil
}
