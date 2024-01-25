package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

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
	router.Post("/{user}", controller.NewProject)
	// Get project information
	router.Get("/{user}/{project}", controller.ProjectInfo)
	// Delete a project
	router.Delete("/{user}/{project}", controller.DeleteProject)
	// Run project build
	router.Post("/{user}/{project}/build", controller.BuildProject)
	// Get list of project source files
	router.Get("/{user}/{project}/src", controller.ListProjectSrc)
	// Create or update project source file
	router.Post("/{user}/{project}/src", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")

		if err := r.ParseMultipartForm(int64(config.MaxFileSize)); err != nil {
			http.Error(w, "Unable to parse form", http.StatusBadRequest)
			log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
			return
		}

		file, fileHeader, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Unable to read form file", http.StatusBadRequest)
			log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
			return
		}
		_ = fileHeader

		path := r.FormValue("path")
		if path == "" {
			http.Error(w, "Unable to read path", http.StatusBadRequest)
			log.Printf("%s %s: %s", r.Method, r.URL.Path, "no file path")
			return
		}

		if err := CreateProjectFile(config, user, project, path, file); err != nil {
			http.Error(w, "Unable to create file", http.StatusInternalServerError)
			log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
			return
		}
	})
	// Retrieve a project source file with the specified hash
	router.Get("/{user}/{project}/src/*", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")
		path := chi.URLParam(r, "*")

		file, err := ReadProjectFile(config, user, project, "src", path)
		if err != nil {
			http.Error(w, "Invalid file path", http.StatusBadRequest)
			log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
			return
		}
		defer file.Close()

		if _, err := io.Copy(w, file); err != nil {
			log.Printf("%s %s copy: %s", r.Method, r.URL.Path, err)
		}
	})
	// Delete a project souce file
	router.Delete("/{user}/{project}/src/*", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")
		path := chi.URLParam(r, "*")

		if err := DeleteProjectFile(config, user, project, "src", path); err != nil {
			http.Error(w, "Failed to delete file", http.StatusBadRequest)
			log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
			return
		}
	})
	// Get a list of project aux files (if created)
	router.Get("/{user}/{project}/aux", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")

		files, err := config.database.ListProjectFiles(user, project, "aux")
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
	// Retrieve a project aux file with the specified hash
	router.Get("/{user}/{project}/aux/*", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")
		path := chi.URLParam(r, "*")

		file, err := ReadProjectFile(config, user, project, "aux", path)
		if err != nil {
			http.Error(w, "Invalid file path", http.StatusBadRequest)
			log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
			return
		}
		defer file.Close()

		if _, err := io.Copy(w, file); err != nil {
			log.Printf("%s %s copy: %s", r.Method, r.URL.Path, err)
		}
	})
	// Get a list of project out files (if created)
	router.Get("/{user}/{project}/out", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")

		files, err := config.database.ListProjectFiles(user, project, "out")
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
	// Retrieve a project out file with the specified hash
	router.Get("/{user}/{project}/out/*", func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "user")
		project := chi.URLParam(r, "project")
		path := chi.URLParam(r, "*")

		file, err := ReadProjectFile(config, user, project, "out", path)
		if err != nil {
			http.Error(w, "Invalid file path", http.StatusBadRequest)
			log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
			return
		}
		defer file.Close()

		if _, err := io.Copy(w, file); err != nil {
			log.Printf("%s %s copy: %s", r.Method, r.URL.Path, err)
		}
	})
}
