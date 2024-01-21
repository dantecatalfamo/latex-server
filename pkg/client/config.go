package client

import "github.com/dantecatalfamo/remotex/pkg/server"

type GlobalConfig struct {
	user string
	serverBaseUrl string
}

type RepoConfig struct {
	BuildOptions server.ProjectBuildOptions
	ProjectName string
	SaveAuxFiles bool
}
