package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func SetupRoutes(config Config, router *chi.Mux) {
	// List projects
	router.Get("/projects", func(w http.ResponseWriter, r *http.Request) {})
	// Create new project with randomly generated ID
	router.Post("/projects", func(w http.ResponseWriter, r *http.Request) {
		type NewProjectResponse struct {
			Id string `json:"id"`
		}
		projectId, err := NewProject(config, "")
		if err != nil {
			http.Error(w, "Failed to create new project", http.StatusInternalServerError)
			log.Printf("POST /projects: %s", err)
			return
		}
		log.Printf("new project: %s", projectId)
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(NewProjectResponse{ Id: projectId })
		if err != nil {
			http.Error(w, "Failed to serialize json", http.StatusInternalServerError )
			log.Printf("POST /projects: %s", err)
			return
		}
	})
	// Get project information
	router.Get("/project/{projectName}", func(w http.ResponseWriter, r *http.Request) {
		projectId := chi.URLParam(r, "projectName")
		if !ValidateProjectId(projectId) {
			http.Error(w, "Invalid project ID", http.StatusBadRequest)
			log.Printf("Invalid project id: %s", projectId)
			return
		}

		info, err := ReadProjectInfo(config, projectId)
		if err != nil {
			http.Error(w, "Failed to read projet into", http.StatusInternalServerError)
			log.Printf("GET /project/%s: %s", projectId, err)
			return
		}
		err = json.NewEncoder(w).Encode(info)
		if err != nil {
			http.Error(w, "Failed to serialize json", http.StatusInternalServerError)
			log.Printf("GET /project/%s: %s", projectId, err)
			return
		}
	})
	// Delete a project
	router.Delete("/project/{projectName}", func(w http.ResponseWriter, r *http.Request) {})
	// Run project build
	router.Post("/project/{projectName}/build", func(w http.ResponseWriter, r *http.Request) {})
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
