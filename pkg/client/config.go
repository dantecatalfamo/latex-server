package client

import "github.com/dantecatalfamo/remotex/pkg/server"

type GlobalConfig struct {
	User string `json:"user"`
	ServerBaseUrl string `json:"serverBaseUrl"`
}

type RepoConfig struct {
	BuildOptions server.ProjectBuildOptions `json:"buildOptions"`
	ProjectName string `json:"projectName"`
	SaveAuxFiles bool `json:"saveAuxFiles"`
}
