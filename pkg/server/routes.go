package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"

	"github.com/go-chi/chi/v5"
)

func SetupRoutes(config Config, router *chi.Mux) {
	// TODO Add route to list builds/get specific/latest build info
	// Maybe /user/project/builds (list)
	//       /user/project/builds/(id|latest) (build info)
	// TODO Authenticate routes, check if public, etc.
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
		user := chi.URLParam(r, "user")
		r.ParseForm()
		project := r.FormValue("project")
		if project == "" {
			http.Error(w, "Missing project name", http.StatusBadRequest)
			return
		}
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
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "404 page not found", http.StatusBadRequest)
			} else {
				http.Error(w, "Failed to retrieve project information", http.StatusInternalServerError)
				log.Printf("GET /%s/%s: %s", user, project, err)
			}
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
		case string(EnginePDF):
			engine = EnginePDF
		case string(EngineLua):
			engine = EngineLua
		case string(EngineXeTeX):
			engine = EngineXeTeX
		}

		options := ProjectBuildOptions{
			Force: r.Form.Has("force"),
			FileLineError: r.Form.Has("fileLineError"),
			Document: r.Form.Get("document"),
			Engine: engine,
			Dependents: r.Form.Has("dependents"),
			CleanBuild: r.Form.Has("cleanBuild"),
		}

		log.Printf("Build started: %s/%s %+v", user, project, options)

		stdout, err := BuildProject(context.Background(), config, user, project, options)
		if err != nil {
			// If the error was the child process, return the output
			var execErr *exec.ExitError
			if errors.As(err, &execErr) {
				http.Error(w, stdout, http.StatusUnprocessableEntity)
			} else {
				http.Error(w, "Unable to build project", http.StatusInternalServerError)
			}
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

		files, err := config.Database.ListProjectFiles(user, project, "src")
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

		files, err := config.Database.ListProjectFiles(user, project, "aux")
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

		files, err := config.Database.ListProjectFiles(user, project, "out")
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
