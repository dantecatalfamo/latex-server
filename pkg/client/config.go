package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/dantecatalfamo/remotex/pkg/server"
)

type GlobalConfig struct {
	User string `json:"user"`
	ServerBaseUrl string `json:"serverBaseUrl"`
}

type ProjectConfig struct {
	BuildOptions server.ProjectBuildOptions `json:"buildOptions"`
	ProjectName string `json:"projectName"`
	SaveAuxFiles bool `json:"saveAuxFiles"`
}

func ReadGlobalConfig() (GlobalConfig, error) {
	configPath, err := xdg.ConfigFile("remotex/remotex.json")
	if err != nil {
		return GlobalConfig{}, fmt.Errorf("ReadGlobalConfig create path: %w", err)
	}
	file, err := os.Open(configPath)
	if err != nil {
		return GlobalConfig{}, fmt.Errorf("ReadGlobalConfig open file: %w", err)
	}
	defer file.Close()

	var globalConfig GlobalConfig
	if err := json.NewDecoder(file).Decode(&globalConfig); err != nil {
		return GlobalConfig{}, fmt.Errorf("ReadGlobalConfig decode: %w", err)
	}

	return globalConfig, nil
}

func WriteGlobalConfig(globalConfig GlobalConfig) error {
	configPath, err := xdg.ConfigFile("remotex/remotex.json")
	if err != nil {
		return fmt.Errorf("WriteGlobalConfig create path: %w", err)
	}
	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("WriteGlobalConfig open file: %w", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(globalConfig); err != nil {
		return fmt.Errorf("WriteGlobalConfig decode: %w", err)
	}

	return nil
}

// TODO Make project name and id different?
// Make IDs random strings and the name is the user readable name?

func ReadProjectConfig(projectRoot string) (ProjectConfig, error) {
	configPath := filepath.Join(projectRoot, ProjectConfigName)

	file, err := os.Open(configPath)
	if err != nil {
		return ProjectConfig{}, fmt.Errorf("ReadProjectConfig open file: %w", err)
	}
	defer file.Close()

	var projectConfig ProjectConfig

	if err := json.NewDecoder(file).Decode(&projectConfig); err != nil {
		return ProjectConfig{}, fmt.Errorf("ReadProjectConfig decode json: %w", err)
	}

	return projectConfig, nil
}

func WriteProjectConfig(projectRoot string, projectConfig ProjectConfig) error {
	configPath := filepath.Join(projectRoot, ProjectConfigName)

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("WriteProjectConfig create file: %w", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(projectConfig); err != nil {
		return fmt.Errorf("WriteProjectConfig encode: %w", err)
	}

	return nil
}
