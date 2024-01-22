package client

import "github.com/dantecatalfamo/remotex/pkg/server"

type GlobalConfig struct {
	User string `json:"user"`
	ServerBaseUrl string `json:"serverBaseUrl"`
}

type ProjectConfig struct {
	BuildOptions server.ProjectBuildOptions `json:"buildOptions"`
	ProjectName string `json:"projectName"`
	SaveAuxFiles bool `json:"saveAuxFiles"`
}

// TODO Make project name and id different?
// Make IDs random strings and the name is the user readable name?
