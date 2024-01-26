package server

import (
	"github.com/go-chi/chi/v5"
)

func SetupRoutes(config Config, router *chi.Mux) {
	// TODO Add route to list builds/get specific/latest build info
	// Maybe /user/project/builds (list)
	//       /user/project/builds/(id|latest) (build info)
	// TODO Authenticate routes, check if public, etc.
	controller := NewController(config)

	router.Use(TokenAuthMiddleware(config))

	router.Route("/{user}", func(rUser chi.Router) {
		// List projects
		rUser.Get("/", controller.ListProjects)
		// Create a new project
		rUser.Post("/", controller.CreateProject)
		// Get project information
		rUser.Route("/{project}", func(rProject chi.Router) {
			rProject.Get("/", controller.ProjectInfo)
			// Delete a project
			rProject.Delete("/", controller.DeleteProject)
			// Run project build
			rProject.Post("/build", controller.BuildProject)
			// Get list of project source files
			rProject.Get("/src", controller.ListSrcFiles)
			// Create or update project source file
			rProject.Post("/src", controller.CreateSrcFile)
			// Retrieve a project source file with the specified hash
			rProject.Get("/src/*", controller.ReadSrcFile)
			// Delete a project souce file
			rProject.Delete("/src/*", controller.DeleteSrcFile)
			// Get a list of project aux files (if created)
			rProject.Get("/aux", controller.ListAuxFiles)
			// Retrieve a project aux file with the specified hash
			rProject.Get("/aux/*", controller.ReadAuxFile)
			// Get a list of project out files (if created)
			rProject.Get("/out", controller.ListOutFiles)
			// Retrieve a project out file with the specified hash
			rProject.Get("/out/*", controller.ReadOutFile)

		})

	})
}
