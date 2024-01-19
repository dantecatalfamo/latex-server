package main

import (
	"context"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// NewProject creates a new project belonging to owner with name, and
// creates the appropriate subdirectories
func NewProject(config Config, user string, name string) error {
	userId, err := config.Database.GetUserId(user)
	if err != nil {
		return fmt.Errorf("NewProject: %w", err)
	}

	if _, err := config.Database.conn.Exec("INSERT INTO projects (name, user_id) VALUES (?, ?)", name, userId); err != nil {
		return fmt.Errorf("NewProject: %w", err)
	}

	projectPath := filepath.Join(config.ProjectDir, user, name)
	if err := os.Mkdir(projectPath, 0700); err != nil {
		return fmt.Errorf("NewProject Mkdir: %w", err)
	}

	for _, subdir := range([]string{"aux", "out", "src"}) {
		subDirPath := filepath.Join(projectPath, subdir)
		if err := os.Mkdir(subDirPath, 0700); err != nil {
			return fmt.Errorf("NewProject subdir: %w", err)
		}
	}

	return nil
}

type FileInfo struct {
	Path string        `json:"path"`
	Size uint64        `json:"size"`
	Sha512Hash string  `json:"sha512hash"`
}

// ListProjectFiles returns a list of files in the subdir of a project
// directory.
func ListProjectFiles(config Config, user string, projectName string, subdir string) ([]FileInfo, error) {
	projectId, err := config.Database.GetProjectId(user, projectName)
	if err != nil {
		return nil, fmt.Errorf("ListProjectFiles: %w", err)
	}

	rows, err := config.Database.conn.Query("SELECT path, size, sha512hash FROM files WHERE project_id = ?", projectId)
	if err != nil {
		return nil, fmt.Errorf("ListProjectFiles: %w", err)
	}

	defer rows.Close()

	var fileInfo []FileInfo

	for rows.Next() {
		info := FileInfo{}
		if err := rows.Scan(&info.Path, &info.Size, &info.Sha512Hash); err != nil {
			return nil, fmt.Errorf("ListProjectFiles row scan: %w", err)
		}
		fileInfo = append(fileInfo, info)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("ListProjectFiles rows error: %w", rows.Err())
	}

	return fileInfo, nil
}

// ScanProjectFiles deletes the file list from a project's subdir and
// re-scans them
func ScanProjectFiles(config Config, user string, projectName string, subdir string) error {
	projectPath := filepath.Join(config.ProjectDir, user, projectName)
	filesPath := filepath.Join(projectPath, subdir)

	projectId, err := config.Database.GetProjectId(user, projectName)
	if err != nil {
		return fmt.Errorf("ScanProjectFiles: %w", err)
	}

	tx, err := config.Database.conn.Begin()
	if err != nil {
		return fmt.Errorf("ScanProjectFiles begin transaction: %w", err)
	}

	if _, err := tx.Exec("DELETE FROM files WHERE project_id = ? AND subdir = ?", projectId, subdir); err != nil {
		tx.Rollback()
		return fmt.Errorf("ScanProjectFiles deleting files cache: %w", err)
	}

	err = filepath.Walk(filesPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			log.Printf("ScanProjectFiles of \"%s\", path \"%s\": %s", filesPath, path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}

		size := info.Size()
		fileData, err := os.ReadFile(path)
		hash := sha512.Sum512(fileData)
		digest := fmt.Sprintf("%x", hash)
		partialPath := strings.TrimPrefix(path, filesPath)

		if _, err := tx.Exec(
			"INSERT INTO files (path, size, sha512sum) VALUES (?, ?, ?)",
			partialPath,
			size,
			digest,
		); err != nil {
			return fmt.Errorf("ScanProjectFiles insert row: %w", err)
		}

		return nil
	})

	if err != nil {
		tx.Rollback()
		return fmt.Errorf("ScanProjectFiles walk: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("ScanProjectFiles commit transaction: %w", err)
	}

	return nil
}

// ClearProjectDir empties a project's subdirectory. This would
// usually be something like src, aux, or out.
func ClearProjectDir(config Config, user string, projectName string, subdir string) error {
	projectPath := filepath.Join(config.ProjectDir, user, projectName)
	subdirPath := filepath.Join(projectPath, subdir)

	projectId, err := config.Database.GetProjectId(user, projectName)
	if err != nil {
		return fmt.Errorf("ClearProjectDir: %w", err)
	}

	subdirFile, err := os.Open(subdirPath)
    if err != nil {
        return fmt.Errorf("ClearProjectDir opening subdir: %w", err)
    }
    defer subdirFile.Close()

    names, err := subdirFile.Readdirnames(-1)
    if err != nil {
        return fmt.Errorf("CleanProjectDir reading subdir: %w", err)
    }

    for _, name := range names {
        err = os.RemoveAll(filepath.Join(subdirPath, name))
        if err != nil {
            return fmt.Errorf("ClearProjectDir: %w", err)
        }
    }

	if _, err = config.Database.conn.Exec("DELETE FROM files WHERE project_id = ? AND subdir = ?", projectId, subdir); err != nil {
		return fmt.Errorf("ClearProjectDir deleting files cache: %w", err)
	}

	return nil
}

// Options for BuildProject
type ProjectBuildOptions struct {
	Force bool `json:"force"` // Run latex in nonstop mode, and latexmk with force flag
	FileLineError bool `json:"fileLineError"` // Erorrs are in c-style file:line:error format
	Engine Engine `json:"engine"` // LaTeX engine to use
	Document string `json:"document"` // The name of the main document
	Dependents bool `json:"dependents"` // List dependent files in build output
}

// BuildProject builds a project using latexmk using the options
// provided. It retuens the stdout of latexmk.
func BuildProject(ctx context.Context, config Config, user string, projectName string, options ProjectBuildOptions) (string, error) {
	projectPath := filepath.Join(config.ProjectDir, user, projectName)
	srcPath := filepath.Join(projectPath, "src")
	outPath := filepath.Join(projectPath, "out")
	auxPath := filepath.Join(projectPath, "aux")

	projectId, err := config.Database.GetProjectId(user, projectName)
	if err != nil {
		return "", fmt.Errorf("BuildProject: %w", err)
	}

	if err := ClearProjectDir(config, user, projectName, "aux"); err != nil {
		return "", fmt.Errorf("BuildProject clearing %s/%s/aux: %w", user, projectName, err)
	}

	if err := ClearProjectDir(config, user, projectName, "out"); err != nil {
		return "", fmt.Errorf("BuildProject clearing %s/%s/out: %w", user, projectName, err)
	}

	opts, err := json.Marshal(options)
	if err != nil {
		return "", fmt.Errorf("BuildProject marshall: %w", err)
	}
	result, err := config.Database.conn.Exec("INSERT INTO builds (project_id, status, options) VALUES (?, ?, ?)", projectId, "running", opts)
	if err != nil {
		return "", fmt.Errorf("BuildProject db insert build: %w", err)
	}

	buildId, err := result.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("BuildProject get buildId: %w", err)
	}

	beginTime := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, config.MaxProjectBuildTime)

	// If error is type *ExitError, the cmdOut should be populated
	// with an error message
	buildOut, err := RunBuild(timeoutCtx, BuildOptions{
		AuxDir: auxPath,
		OutDir: outPath,
		SrcDir: srcPath,
		// SharedDir: "", // TODO Make shared dir work
		Document: options.Document,
		Engine: options.Engine,
		Force: options.Force,
		FileLineError: options.FileLineError,
		Dependents: options.Dependents,
	})
	buildTime := time.Since(beginTime)
	cancel() // Don't leak the context

	buildOut = strings.ReplaceAll(buildOut, projectPath, "<project>")

	if err != nil {
		reason := "(internal)"
		var execErr *exec.ExitError
		if errors.As(err, &execErr) {
			reason = fmt.Sprintf("(%d)", execErr.ExitCode())
		}
		if _, err := config.Database.conn.Exec(
			"UPDATE builds SET status = ?, build_time = ?, build_out = ? WHERE id = ?",
			fmt.Sprintf("failed %s", reason),
			buildTime.Seconds(),
			buildOut,
			buildId,
		); err != nil {
			return buildOut, fmt.Errorf("BuildProject updating db failed build: %w", err)
		}

		return buildOut, fmt.Errorf("BuildProject failed build: %w", err)
	}

	if _, err := config.Database.conn.Exec(
		"UPDATE builds SET status = 'finished', build_time = ?, build_out = ? WHERE id = ?",
		buildTime.Seconds(),
		buildOut,
		buildId,
	); err != nil {
		return buildOut, fmt.Errorf("BuildProject updating db finished build: %w", err)
	}

	if err := ScanProjectFiles(config, user, projectName, "aux"); err != nil {
		return buildOut, fmt.Errorf("BuildProject scan aux: %w", err)
	}

	if err := ScanProjectFiles(config, user, projectName, "out"); err != nil {
		return buildOut, fmt.Errorf("BuildProject scan out: %w", err)
	}

	return buildOut, nil
}

func DeleteProject(config Config, user string, projectName string) error {
	projectPath := filepath.Join(config.ProjectDir, user, projectName)

	projectId, err := config.Database.GetProjectId(user, projectName)
	if err != nil {
		return fmt.Errorf("DeleteProject get projec id: %w", err)
	}

	if err := os.RemoveAll(projectPath); err != nil {
		return fmt.Errorf("DeleteProject: %w", err)
	}

	if _, err := config.Database.conn.Exec("DELETE FROM projects WHERE id = ?", projectId); err != nil {
		return fmt.Errorf("DeleteProject delete db project: %w", err)
	}

	return nil
}
