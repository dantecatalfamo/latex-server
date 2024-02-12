package server

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("allowLatexmkrc", false)
	viper.SetDefault("allowLuaTex", false)
	viper.SetDefault("buildMode", BuildModeNative)
	viper.SetDefault("databasePath", "/var/db/remotex/remotex.db")
	viper.SetDefault("listenAddress", "0.0.0.0:3344")
	viper.SetDefault("maxBuildTime", "45s")
	viper.SetDefault("maxFileSize", 25 * 1024 * 1024)
	viper.SetDefault("projectsPath", "/var/lib/remotex/")

	viper.SetConfigName("remotex")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/remotex/")
}

type Config struct {
	AllowLatexmkrc bool // Allow auto-reading latexmkrc files
	AllowLuaTex bool // Allow luaTex, possible security issue for some
	BuildMode BuildMode // Select between native or containerized builds
	DatabasePath string // Location of the database
	ListenAddress string // Where the server will listen
	MaxFileSize uint // Maximum upload size
	MaxProjectBuildTime time.Duration // Max time a project can build
	ProjectDir string // Root of all projects
	database *Database // Database object
}

type BuildMode string

const BuildModeNative BuildMode = "native"
const BuildModeDocker BuildMode = "docker"

// WriteNewConfig creates a new config with default values and writes
// it to a file at path
func WriteNewConfig(path string) error {
	if err := viper.WriteConfigAs(path); err != nil {
		return fmt.Errorf("WriteNewConfig: %w", err)
	}

	return nil
}

func SetExplicitConfigFile(path string) {
	viper.SetConfigFile(path)
}

// ReadAndInitializeConfig reads a config at path and does everything
// required to use the config. This includes opening and migrating the
// databse if required, and creating directories.
func ReadAndInitializeConfig(path string) (Config, error) {
	var config Config
	if err := viper.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig read in: %w", err)
	}

	maxProjectBuildTime, err := time.ParseDuration(viper.GetString("maxBuildTime"))
	if err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig parse max build time: %w", err)
	}

	var buildMode BuildMode
	switch strMode := viper.GetString("buildMode"); strMode {
	case string(BuildModeNative):
		buildMode = BuildModeNative
	case string(BuildModeDocker):
		buildMode = BuildModeDocker
	default:
		return Config{}, fmt.Errorf("ReadAndInitializeConfig invalid build mode: %s", strMode)
	}

	config.AllowLatexmkrc = viper.GetBool("allowLatexmkrc")
	config.AllowLuaTex = viper.GetBool("allowLuaTex")
	config.BuildMode = buildMode
	config.DatabasePath = viper.GetString("databasePath")
	config.ListenAddress = viper.GetString("listenAddress")
	config.MaxFileSize = viper.GetUint("maxFileSize")
	config.MaxProjectBuildTime = maxProjectBuildTime
	config.ProjectDir = viper.GetString("projectsPath")

	if err := os.MkdirAll(config.ProjectDir, os.ModePerm); err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig create project dir: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(config.DatabasePath), os.ModePerm); err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig create db path: %w", err)
	}

	db, err := NewDatabse(config.DatabasePath)
	if err != nil {
		return Config{}, fmt.Errorf("ReadAndInitializeConfig open db: %w", err)
	}

	config.database = db

	return config, nil
}
