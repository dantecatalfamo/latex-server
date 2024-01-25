package server

import (
	"github.com/go-chi/chi/v5"
)

func SetupRoutes(config Config, router *chi.Mux) {
	// TODO Add route to list builds/get specific/latest build info
	// Maybe /user/project/builds (list)
	//       /user/project/builds/(id|latest) (build info)
	// TODO Authenticate routes, check if public, etc.
	// List projects
	controller := NewController(config)
	router.Get("/{user}", controller.ListProjects)
	// Create a new project
	router.Post("/{user}", controller.CreateProject)
	// Get project information
	router.Get("/{user}/{project}", controller.ProjectInfo)
	// Delete a project
	router.Delete("/{user}/{project}", controller.DeleteProject)
	// Run project build
	router.Post("/{user}/{project}/build", controller.BuildProject)
	// Get list of project source files
	router.Get("/{user}/{project}/src", controller.ListSrcFiles)
	// Create or update project source file
	router.Post("/{user}/{project}/src", controller.CreateSrcFile)
	// Retrieve a project source file with the specified hash
	router.Get("/{user}/{project}/src/*", controller.ReadSrcFile)
	// Delete a project souce file
	router.Delete("/{user}/{project}/src/*", controller.DeleteSrcFile)
	// Get a list of project aux files (if created)
	router.Get("/{user}/{project}/aux", controller.ListAuxFiles)
	// Retrieve a project aux file with the specified hash
	router.Get("/{user}/{project}/aux/*", controller.ReadAuxFile)
	// Get a list of project out files (if created)
	router.Get("/{user}/{project}/out", controller.ListOutFiles)
	// Retrieve a project out file with the specified hash
	router.Get("/{user}/{project}/out/*", controller.ReadOutFile)
}
