package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TODO Get away from JSON as a config language
// TODO use time.Duration.String and time.ParseDuration
type Config struct {
	ListenAddress string `json:"listenAddress"` // Where the server will listen
	ProjectDir string `json:"projectDir"` // Root of all projects
	MaxProjectBuildTimeString string `json:"maxProjectBuildTime"` // Max time a project can build as a string
	MaxProjectBuildTime time.Duration `json:"-"` // Max time a project can build
	DatabasePath string `json:"databasePath"` // Location of the database
	database *Database // Database object
	MaxFileSize uint `json:"maxFileSize"` // Maximum upload size
	BuildMode BuildMode `json:"buildMode"` // Select between native or containerized builds
	AllowLatexmkrc bool `json:"allowLatexmkrc"` // Allow auto-reading latexmkrc files
	AllowLuaTex bool `json:"allowLuaTeX"` // Allow luaTex, possible security issue for some
}

type BuildMode string

const BuildModeNative BuildMode = "native"
const BuildModeDocker BuildMode = "docker"

// NewConfig returns a Config with default values
func NewConfig() Config {
	return Config{
		ListenAddress: "0.0.0.0:3344",
		BuildMode: BuildModeNative,
		MaxFileSize: 25 * 1024 * 1024,
		MaxProjectBuildTime: 45 * time.Second,
		DatabasePath: "/var/db/remotex/remotex.db",
		ProjectDir: "/var/lib/remotex/",
	}
}

// WriteNewConfig creates a new config with default values and writes
// it to a file at path
func WriteNewConfig(path string) error {
	config := NewConfig()

	config.MaxProjectBuildTimeString = config.MaxProjectBuildTime.String()

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("WriteEmptyConfig create file: %w", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(config); err != nil {
		return fmt.Errorf("WriteEmptyConfig encode: %w", err)
	}

	return nil
}

// ReadAndInitializeConfig reads a config at path and does everything
// required to use the config. This includes opening and migrating the
// databse if required, and creating directories.
func ReadAndInitializeConfig(path string) (Config, error) {
	var config Config
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig read file: %w", err)
	}

	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig decode: %w", err)
	}

	config.MaxProjectBuildTime, err = time.ParseDuration(config.MaxProjectBuildTimeString)
	if err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig parse max build time: %w", err)
	}

	if err := os.MkdirAll(config.ProjectDir, 0700); err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig create project dir: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(config.DatabasePath), 0700); err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig create db path: %w", err)
	}

	db, err := NewDatabse(config.DatabasePath)
	if err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig open db: %w", err)
	}

	config.database = db

	return config, nil
}
