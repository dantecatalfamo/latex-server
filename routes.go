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
	router.Get("/{user}", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		infos, err := config.Database.ListUserProjects(user)
		if err != nil {
			http.Error(w, "No user", http.StatusBadRequest)
			log.Printf("GET %s: %s", r.URL.Path, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(infos)
		if err != nil {
			http.Error(w, "Failed to serialize json", http.StatusInternalServerError)
			log.Printf("GET %s: %s", r.URL.Path, err)
			return
		}
	})
	// Create a new project
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
	router.Delete("/{user}/{project}", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")

		if err := DeleteProject(config, user, project); err != nil {
			http.Error(w, "Unable to delete project", http.StatusInternalServerError)
			log.Printf("DELETE %s: %s", r.URL.Path, err)
			return
		}

		log.Printf("Deleted project: %s/%s", user, project)
	})
	// Run project build
	router.Post("/{user}/{project}/build", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Unable to process request", http.StatusBadRequest)
			log.Printf("POST %s: %s", r.URL.Path, err)
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

		log.Printf("Build started: %s/%s", user, project)

		stdout, err := BuildProject(context.Background(), config, user, project, options)
		if err != nil {
			http.Error(w, stdout, http.StatusUnprocessableEntity)
			log.Printf("POST %s: %s", r.URL.Path, err)
			return
		}

		log.Printf("Build finished: %s/%s", user, project)

		if _, err := fmt.Fprintln(w, stdout); err != nil {
			log.Printf("POST %s: %s", r.URL.Path, err)
		}
	})
	// Get list of project source files
	router.Get("/{user}/{project}/src", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")

		files, err := ListProjectFiles(config, user, project, "src")
		if err != nil {
			http.Error(w, "Failed to list project files", http.StatusInternalServerError)
			log.Printf("GET %s: %s", r.URL.Path, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(files)
		if err != nil {
			http.Error(w, "Failed to serialize json", http.StatusInternalServerError)
			log.Printf("GET %s: %s", r.URL.Path, err)
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
