package server

import "time"

type Config struct {
	ListenAddress string // Where the server will listen
	ProjectDir string // Root of all projects
	MaxProjectBuildTime time.Duration // Max time a project can build
	Database *Database // Database object
	MaxFileSize uint // Maximum upload size
	BuildMode BuildMode // Select between native or containerized builds
	AllowLatexmkrc bool // Allow auto-reading latexmkrc files
}

type BuildMode string

const BuildModeNative = "native"
const BuildModeDocker = "docker"
