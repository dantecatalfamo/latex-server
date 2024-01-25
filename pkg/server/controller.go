package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"

	"github.com/go-chi/chi/v5"
)

type Controller struct {
	config Config
}

func NewController(config Config) Controller {
	return Controller{ config: config }
}

func (c *Controller) ListProjects (w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	infos, err := c.config.database.ListUserProjects(user)
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
}

func (c *Controller) NewProject (w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	r.ParseForm()
	project := r.FormValue("project")
	if project == "" {
		http.Error(w, "Missing project name", http.StatusBadRequest)
		return
	}
	if err := NewProject(c.config, user, project); err != nil {
		http.Error(w, "Failed to create new project", http.StatusInternalServerError)
		log.Printf("POST /%s: %s", user, err)
		return
	}

	log.Printf("New project: %s/%s", user, project)
}

func (c *Controller) ProjectInfo (w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")

	projectInfo, err := c.config.database.GetProjectInfo(user, project)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "404 page not found", http.StatusNotFound)
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
}

func (c *Controller) DeleteProject (w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")

	if err := DeleteProject(c.config, user, project); err != nil {
		http.Error(w, "Unable to delete project", http.StatusInternalServerError)
		log.Printf("DELETE %s: %s", r.URL.Path, err)
		return
	}

	log.Printf("Deleted project: %s/%s", user, project)
}

func (c *Controller) BuildProject (w http.ResponseWriter, r *http.Request) {
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
		CleanBuild: r.Form.Has("cleanBuild"),
		Dependents: r.Form.Has("dependents"),
		Document: r.Form.Get("document"),
		Engine: engine,
		FileLineError: r.Form.Has("fileLineError"),
		Force: r.Form.Has("force"),
	}

	log.Printf("Build started: %s/%s %+v", user, project, options)

	stdout, err := BuildProject(r.Context(), c.config, user, project, options)
	if err != nil {
		// If the error was the child process, return the output
		var execErr *exec.ExitError
		if errors.As(err, &execErr) {
			http.Error(w, stdout, http.StatusUnprocessableEntity)
		} else if errors.Is(err, ErrBuildInProgress) {
			http.Error(w, "Build in progress", http.StatusConflict)
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
}

func (c *Controller) ListProjectSrc (w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")

	files, err := c.config.database.ListProjectFiles(user, project, "src")
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
}
