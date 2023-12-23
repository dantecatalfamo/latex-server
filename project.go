package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func NewProject(config Config, name string) error {
	fullPath := filepath.Join(config.ProjectDir, name)
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		return errors.New("Project already exists")
	}

	if err := os.Mkdir(fullPath, os.ModePerm); err != nil {
		return fmt.Errorf("NewProject Mkdir: %w", err)
	}

	for _, subdir := range([]string{"aux", "out", "src"}) {
		subDirPath := filepath.Join(config.ProjectDir, name, subdir)
		if err := os.Mkdir(subDirPath, os.ModePerm); err != nil {
			return fmt.Errorf("NewProject subdir: %w", err)
		}
	}

	return nil
}
