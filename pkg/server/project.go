package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	Path string       `json:"path"`
	Size uint64       `json:"size"`
	Sha256Sum string  `json:"sha256sum"`
}

// ScanProjectFiles deletes the file list from a project's subdir and
// re-scans them
func ScanProjectFiles(config Config, user string, projectName string, subdir string) error {
	projectPath := filepath.Join(config.ProjectDir, user, projectName)
	filesPath := filepath.Join(projectPath, subdir)
	removePrefix := filesPath + string(filepath.Separator)

	log.Printf("Scanning %s/%s/%s", user, projectName, subdir)

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
		hash := sha256.Sum256(fileData)
		digest := fmt.Sprintf("%x", hash)
		partialPath := strings.TrimPrefix(path, removePrefix)

		if _, err := tx.Exec(
			"INSERT INTO files (project_id, subdir, path, size, sha256sum) VALUES (?, ?, ?, ?, ?)",
			projectId,
			subdir,
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
	CleanBuild bool `json:"cleanBuild"` // Clean aux and out directories before starting the build
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

	if options.CleanBuild {
		if err := ClearProjectDir(config, user, projectName, "aux"); err != nil {
			return "", fmt.Errorf("BuildProject clearing %s/%s/aux: %w", user, projectName, err)
		}

		if err := ClearProjectDir(config, user, projectName, "out"); err != nil {
			return "", fmt.Errorf("BuildProject clearing %s/%s/out: %w", user, projectName, err)
		}
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
	buildOut, buildErr := RunBuild(timeoutCtx, BuildOptions{
		AuxDir: auxPath,
		OutDir: outPath,
		SrcDir: srcPath,
		// SharedDir: "", // TODO Make shared dir work
		Document: options.Document,
		Engine: options.Engine,
		Force: options.Force,
		FileLineError: options.FileLineError,
		Dependents: options.Dependents,
		AllowLatexmkrc: config.AllowLatexmkrc,
	})
	buildTime := time.Since(beginTime)
	cancel() // Don't leak the context

	buildOut = strings.ReplaceAll(buildOut, projectPath, "<project>")

	// If there is an issue with the build, but it's only with the
	// child process (bad input, etc.) we return with the exit code at
	// the bottom, so we have a chance to re-scan the aux and out
	// directories and update the files
	if buildErr != nil {
		var execErr *exec.ExitError
		if errors.As(buildErr, &execErr) {
			// It's a latexmk error
			if _, err := config.Database.conn.Exec(
				"UPDATE builds SET status = ?, build_time = ?, build_out = ? WHERE id = ?",
				fmt.Sprintf("failed (%d)", execErr.ExitCode()),
				buildTime.Seconds(),
				buildOut,
				buildId,
			); err != nil {
				return buildOut, fmt.Errorf("BuildProject updating db exit-code failed build: %w", err)
			}
		} else {
			// Non-latexmk error
			if _, err := config.Database.conn.Exec(
				"UPDATE builds SET status = ?, build_time = ?, build_out = ? WHERE id = ?",
				"failed (internal)",
				buildTime.Seconds(),
				buildOut,
				buildId,
			); err != nil {
				return buildOut, fmt.Errorf("BuildProject updating db internal failed build: %w", err)
			}
		}
	} else {
		if _, err := config.Database.conn.Exec(
			"UPDATE builds SET status = 'finished', build_time = ?, build_out = ? WHERE id = ?",
			buildTime.Seconds(),
			buildOut,
			buildId,
		); err != nil {
			return buildOut, fmt.Errorf("BuildProject updating db finished build: %w", err)
		}
	}

	if err := ScanProjectFiles(config, user, projectName, "aux"); err != nil {
		return buildOut, fmt.Errorf("BuildProject scan aux: %w", err)
	}

	if err := ScanProjectFiles(config, user, projectName, "out"); err != nil {
		return buildOut, fmt.Errorf("BuildProject scan out: %w", err)
	}

	if err := ScanProjectFiles(config, user, projectName, "src"); err != nil {
		return buildOut, fmt.Errorf("BuildProject scan src: %w", err)
	}

	// Finally return the build error if we have one
	if buildErr != nil {
		return buildOut, fmt.Errorf("BuildProject failed build: %w", buildErr)
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

func ReadProjectFile(config Config, user, projectName, subdir, path string) (io.ReadCloser, error) {
	if strings.Contains(path, "../") {
		return nil, errors.New("path contains parent directory traversal")
	}

	projectPath := filepath.Join(config.ProjectDir, user, projectName)
	filePath := filepath.Join(projectPath, subdir, path)

	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("ReadProjectFile stat: %w", err)
	}

	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("file \"%s\" is not a normal file: %s", filePath, stat.Mode().String())
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("ReadProjectFile open file: %w", err)
	}

	return file, nil
}

func DeleteProjectFile(config Config, user, projectName, subdir, path string) error {
	projectPath := filepath.Join(config.ProjectDir, user, projectName)
	filePath := filepath.Join(projectPath, subdir, path)

	projectId, err := config.Database.GetProjectId(user, projectName)
	if err != nil {
		return fmt.Errorf("DeleteProjectFile get project id: %w", err)
	}

	if path == "" || path == "." || path == "./" || path == ".." {
		return errors.New("path is just the top level directory")
	}

	if strings.Contains(path, "../") {
		return errors.New("path contains parent directory traversal")
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("DeleteProjectFile stat: %w", err)
	}

	if stat.IsDir() {
		if err := os.RemoveAll(filePath); err != nil {
			return fmt.Errorf("DeleteProjectFile RemoveAll: %w", err)
		}
		if _, err := config.Database.conn.Exec("DELETE FROM files WHERE project_id = ? ANd subdir = ? AND path GLOB ?",
			projectId,
			subdir,
			fmt.Sprintf("%s/*", path),
		); err != nil {
			return fmt.Errorf("DeleteProjectFile remove dir from db: %w", err)
		}
	} else {
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("DeleteProjectFile Remove: %w", err)
		}
		if _, err := config.Database.conn.Exec(
			"DELETE FROM files WHERE project_id = ? AND subdir = ? AND path = ?",
			projectId,
			subdir,
			path,
		); err != nil {
			return fmt.Errorf("DeleteProjectFile remove file from db: %w", err)
		}
	}

	// Traverse dirs upward and delete any empty dirs until we get to
	// the top of the subdir, or we find a non-empty dir
	dirPath := filepath.Dir(filePath)
	topDirPath := filepath.Join(projectPath, subdir)
	for dirPath != topDirPath {
		empty, err := isDirEmpty(dirPath)
		if err != nil {
			return fmt.Errorf("DeleteProjectFile checking empty dir: %w", err)
		}

		if !empty {
			break
		}

		if err := os.Remove(dirPath); err != nil {
			return fmt.Errorf("DeleteProjectFile clearing empty dirs: %w", err)
		}

		dirPath = filepath.Dir(dirPath)
	}

	return nil
}

func CreateProjectFile(config Config, user, projectName, path string, reader io.Reader) error {
	projectPath := filepath.Join(config.ProjectDir, user, projectName)
	filePath := filepath.Join(projectPath, "src", path)
	fileDir := filepath.Dir(filePath)

	projectId, err := config.Database.GetProjectId(user, projectName)
	if err != nil {
		return fmt.Errorf("CreateProjectFile get project id: %w", err)
	}

	if strings.Contains(path, "../") || strings.Contains(path, "./") {
		return errors.New("path contains parent directory traversal")
	}

	if err := os.MkdirAll(fileDir, 0700); err != nil {
		return fmt.Errorf("CreateProjectFile MkdirAll: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("CreateProjectFile create file: %w", err)
	}

	hasher := sha256.New()
	multiWriter := io.MultiWriter(file, hasher)

	size, err := io.Copy(multiWriter, reader)
	if err != nil {
		return fmt.Errorf("CreateProjectFile copy: %w", err)
	}

	digest := fmt.Sprintf("%x", hasher.Sum(nil))

	if _, err := config.Database.conn.Exec(
		"INSERT INTO files (project_id, subdir, path, size, sha256sum) VALUES (?, ?, ?, ?, ?)",
		projectId,
		"src",
		path,
		size,
		digest,
	); err != nil {
		return fmt.Errorf("CreateProjectFile db insert: %w", err)
	}

	return nil
}

func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdir(1)

	if err == io.EOF {
		return true, nil
	}
	return false, err
}
