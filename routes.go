package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func SetupRoutes(router *chi.Mux) {
	// List projects
	router.Get("/projects", func(w http.ResponseWriter, r *http.Request) {})
	// Create new project with randomly generated ID
	router.Post("/projects", func(w http.ResponseWriter, r *http.Request) {})
	// Get project information
	router.Get("/project/{projectName}", func(w http.ResponseWriter, r *http.Request) {})
	// Delete a project
	router.Delete("/project/{projectName}", func(w http.ResponseWriter, r *http.Request) {})
	// Run project build
	router.Post("/project/build", func(w http.ResponseWriter, r *http.Request) {})
	// Get list of project source files
	router.Get("/project/{projectName}/src", func(w http.ResponseWriter, r *http.Request) {})
	// Create or update project source file
	router.Post("/project/{projectName}/src", func(w http.ResponseWriter, r *http.Request) {})
	// Retrieve a project source file with the specified hash
	router.Get("/project/{projectName}/src/{fileHash}", func(w http.ResponseWriter, r *http.Request) {})
	// Delete a project souce file with the specified hash
	router.Delete("/project/{projectName}/src/{fileHash}", func(w http.ResponseWriter, r *http.Request) {})
	// Get a list of project aux files (if created)
	router.Get("/project/{projectName}/aux", func(w http.ResponseWriter, r *http.Request) {})
	// Retrieve a project aux file with the specified hash
	router.Get("/project/{projectName}/aux/{fileHash}", func(w http.ResponseWriter, r *http.Request) {})
	// Get a list of project out files (if created)
	router.Get("/project/{projectName}/out", func(w http.ResponseWriter, r *http.Request) {})
	// Retrieve a project out file with the specified hash
	router.Get("/project/{projectName}/aux/{fileHash}", func(w http.ResponseWriter, r *http.Request) {})
}
