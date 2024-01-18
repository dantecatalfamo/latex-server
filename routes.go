package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func SetupRoutes(config Config, router *chi.Mux) {
	// TODO Add route to list builds/get specific/latest build info
	// Maybe /user/project/builds (list)
	//       /user/project/builds/(id|latest) (build info)
	// TODO Authenticate routes
	// List projects
	// router.Get("/projects", func(w http.ResponseWriter, r *http.Request) {})
	// Create new project with randomly generated ID
	router.Post("/{user}", func(w http.ResponseWriter, r *http.Request) {
		// TODO get new project name from POST form
		project := "test"
		user := chi.URLParam(r, "user")
		if err := NewProject(config, user, project); err != nil {
			http.Error(w, "Failed to create new project", http.StatusInternalServerError)
			log.Printf("POST /%s: %s", user, err)
			return
		}

		log.Printf("New project: %s/%s", user, project)
		// w.Header().Set("Content-Type", "application/json")
	})
	// Get project information
	router.Get("/{user}/{project}", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")

		projectInfo, err := config.Database.GetProjectInfo(user, project)
		if err != nil {
			http.Error(w, "Failed to retrieve project information", http.StatusInternalServerError)
			log.Printf("GET /%s/%s: %s", user, project, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(projectInfo)
		if err != nil {
			http.Error(w, "Failed to serialize json", http.StatusInternalServerError)
			log.Printf("GET /%s/%s: %s", user, project, err)
			return
		}
	})
	// Delete a project
	router.Delete("/project/{projectName}", func(w http.ResponseWriter, r *http.Request) {
		projectId := chi.URLParam(r, "projectName")
		if !ValidateProjectId(config, projectId) {
			http.Error(w, "Invalid project ID", http.StatusBadRequest)
			log.Printf("Invalid project id: %s", projectId)
			return
		}

		log.Printf("Delete project: %s", projectId)
		err := DeleteProject(config, projectId)
		if err != nil {
			http.Error(w, "Invalid project ID", http.StatusBadRequest)
			log.Printf("DELETE /project/%s: %s", projectId, err)
		}
	})
	// Run project build
	router.Post("/project/{projectName}/build", func(w http.ResponseWriter, r *http.Request) {
		projectId := chi.URLParam(r, "projectName")
		if !ValidateProjectId(config, projectId) {
			http.Error(w, "Invalid project ID", http.StatusBadRequest)
			log.Printf("Invalid project id: %s", projectId)
			return
		}

		if err := ClearProjectDir(config, projectId, "aux"); err != nil {
			http.Error(w, "Failed to build", http.StatusInternalServerError)
			log.Printf("POST /project/%s/build: %s", projectId, err)
			return
		}
		if err := ClearProjectDir(config, projectId, "out"); err != nil {
			http.Error(w, "Failed to build", http.StatusInternalServerError)
			log.Printf("POST /project/%s/build: %s", projectId, err)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Unable to process request", http.StatusBadRequest)
			log.Printf("POST /project/%s/build: %s", projectId, err)
			return
		}

		var engine Engine
		switch r.Form.Get("engine") {
		case "pdf":
			engine = EnginePDF
		case "lua":
			engine = EngineLua
		case "xe":
			engine = EngineXeTeX
		}

		options := ProjectBuildOptions{
			Force: r.Form.Has("force"),
			FileLineError: r.Form.Has("fileLineError"),
			Document: r.Form.Get("document"),
			Engine: engine,
		}

		log.Printf("Build started: %s", projectId)

		stdout, err := BuildProject(context.Background(), config, projectId, options)
		if err != nil {
			http.Error(w, stdout, http.StatusUnprocessableEntity)
			log.Printf("POST /project/%s/build: %s", projectId, err)
			return
		}

		log.Printf("Build finished: %s", projectId)

		if _, err := fmt.Fprintln(w, stdout); err != nil {
			log.Printf("POST /project/%s/build: %s", projectId, err)
		}
	})
	// Get list of project source files
	router.Get("/project/{projectName}/src", func(w http.ResponseWriter, r *http.Request) {
		projectId := chi.URLParam(r, "projectName")
		if !ValidateProjectId(config, projectId) {
			http.Error(w, "Invalid project ID", http.StatusBadRequest)
			log.Printf("Invalid project id: %s", projectId)
			return
		}

		files, err := ListProjectFiles(config, projectId, "src")
		if err != nil {
			http.Error(w, "Failed to list project files", http.StatusInternalServerError)
			log.Printf("GET /project/%s/src: %s", projectId, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(files)
		if err != nil {
			http.Error(w, "Failed to serialize json", http.StatusInternalServerError)
			log.Printf("GET /project/%s/src: %s", projectId, err)
		}
	})
	// Create or update project source file
	router.Post("/project/{projectName}/src", func(w http.ResponseWriter, r *http.Request) {})
	// Retrieve a project source file with the specified hash
	router.Get("/project/{projectName}/src/{fileHash}", func(w http.ResponseWriter, r *http.Request) {})
	// Delete a project souce file with the specified hash
	// TODO maybe this should be a POST endpoint that accepts a list
	// of files so we don't have to keep re-creating the hash index if
	// we want to delete multiple files... or maybe just remove them
	// from the cache list and re-save it.
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
