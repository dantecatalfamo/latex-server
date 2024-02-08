package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Controller struct {
	config Config
}

func NewController(config Config) Controller {
	return Controller{ config: config }
}

func (c *Controller) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "unable to parse form", http.StatusBadRequest)
		log.Printf("%s %s: %s", r.Method, r.URL, err)
		return
	}

	user := r.FormValue("username")
	password := r.FormValue("password")
	description := r.FormValue("description")

	if err := CompareUserPassword(c.config, user, password); err != nil {
		http.Error(w, "incorrect username or password", http.StatusUnauthorized)
		log.Printf("%s %s: %s", r.Method, r.URL, err)
		return
	}

	token, err := CreateUserToken(c.config, user, description)
	if err != nil {
		http.Error(w, "error creating token", http.StatusInternalServerError)
		log.Printf("%s %s: %s", r.Method, r.URL, err)
		return
	}

	fmt.Fprintln(w, token)
}

func (c *Controller) Logout(w http.ResponseWriter, r *http.Request) {
	token := GetAuthToken(r.Context())
	if token == "" {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		log.Printf("%s %s: not logged in", r.Method, r.URL)
		return
	}

	if err := DeleteUserToken(c.config, token); err != nil {
		http.Error(w, "error deleting token", http.StatusInternalServerError)
		log.Printf("%s %s: %s", r.Method, r.URL, err)
		return
	}
}

func (c *Controller) ListProjects(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	infos, err := c.config.database.ListUserProjects(user)
	if err != nil {
		http.Error(w, "404 page not found", http.StatusNotFound)
		log.Printf("GET %s: %s", r.URL.Path, err)
		return
	}

	var userInfo UserInfo

	// If we are not the authorized, only return public projects
	if IsUserAuthed(r.Context(), user) {
		userInfo = UserInfo{ Name: user, Projects: infos }
	} else {
		userInfo.Name = user
		for _, project := range infos {
			if project.Public {
				userInfo.Projects = append(userInfo.Projects, project)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(userInfo)
	if err != nil {
		http.Error(w, "Failed to serialize json", http.StatusInternalServerError)
		log.Printf("GET %s: %s", r.URL.Path, err)
		return
	}
}

func (c *Controller) CreateProject(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	if !IsUserAuthed(r.Context(), user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		log.Printf("%s %s: %s", r.Method, r.URL.Path, "forbidden")
		return
	}

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

func (c *Controller) ProjectInfo(w http.ResponseWriter, r *http.Request) {
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

func (c *Controller) DeleteProject(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")

	if err := DeleteProject(c.config, user, project); err != nil {
		http.Error(w, "Unable to delete project", http.StatusInternalServerError)
		log.Printf("DELETE %s: %s", r.URL.Path, err)
		return
	}

	log.Printf("Deleted project: %s/%s", user, project)
}

func (c *Controller) BuildProject(w http.ResponseWriter, r *http.Request) {
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

	requestId := middleware.GetReqID(r.Context())

	log.Printf("[%s] Build started: %s/%s %+v", requestId, user, project, options)

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

	log.Printf("[%s] Build finished: %s/%s", requestId, user, project)

	if _, err := fmt.Fprintln(w, stdout); err != nil {
		log.Printf("POST %s: %s", r.URL.Path, err)
	}
}

func (c *Controller) ListSrcFiles(w http.ResponseWriter, r *http.Request) {
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

func (c *Controller) CreateSrcFile(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")

	if err := r.ParseMultipartForm(int64(c.config.MaxFileSize)); err != nil {
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

	if err := CreateProjectFile(c.config, user, project, path, file); err != nil {
		http.Error(w, "Unable to create file", http.StatusInternalServerError)
		log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
		return
	}
}

func (c *Controller) ReadSrcFile(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")
	path := chi.URLParam(r, "*")

	file, err := ReadProjectFile(c.config, user, project, "src", path)
	if err != nil {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
		return
	}
	defer file.Close()

	if _, err := io.Copy(w, file); err != nil {
		log.Printf("%s %s copy: %s", r.Method, r.URL.Path, err)
	}
}

func (c *Controller) DeleteSrcFile(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")
	path := chi.URLParam(r, "*")

	if err := DeleteProjectFile(c.config, user, project, "src", path); err != nil {
		http.Error(w, "Failed to delete file", http.StatusBadRequest)
		log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
		return
	}
}

func (c *Controller) ListAuxFiles(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")

	files, err := c.config.database.ListProjectFiles(user, project, "aux")
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

func (c *Controller) ReadAuxFile(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")
	path := chi.URLParam(r, "*")

	file, err := ReadProjectFile(c.config, user, project, "aux", path)
	if err != nil {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
		return
	}
	defer file.Close()

	if _, err := io.Copy(w, file); err != nil {
		log.Printf("%s %s copy: %s", r.Method, r.URL.Path, err)
	}
}

func (c *Controller) ListOutFiles(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")

	files, err := c.config.database.ListProjectFiles(user, project, "out")
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

func (c *Controller) ReadOutFile(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	project := chi.URLParam(r, "project")
	path := chi.URLParam(r, "*")

	file, err := ReadProjectFile(c.config, user, project, "out", path)
	if err != nil {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
		return
	}
	defer file.Close()

	if _, err := io.Copy(w, file); err != nil {
		log.Printf("%s %s copy: %s", r.Method, r.URL.Path, err)
	}
}
