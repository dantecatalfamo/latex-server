package main

import (
	"context"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The Project ID is a hex representation of 24 random bytes
const ProjectIdByteLength = 24

// We should be using a database like SQLite3 for this at some point,
// but for now we're just playing around
type ProjectInfo struct {
	Name string          `json:"name"`
	Owner string         `json:"owner"`
	CreatedAt time.Time  `json:"createdAt"`
	LastBuild time.Time  `json:"lastBuild"`
	BuildTime string     `json:"buildTime"`
	BuildStatus string   `josn:"buildStatus"`
}

// ValidateProjectId checks if projectId is a valid ProjectID string
func ValidateProjectId(config Config, projectId string) bool {
	if len(projectId) != ProjectIdByteLength * 2 {
		return false
	}
	dst := make([]byte, hex.DecodedLen(len(projectId)))
	if _, err := hex.Decode(dst, []byte(projectId)); err != nil {
		return false
	}

	projectPath := filepath.Join(config.ProjectDir, projectId)
	if _, err := os.Stat(projectPath); err != nil {
		return false
	}
	return true
}

// ReadProjectInfo reads the ProjectInfo of a project.
func ReadProjectInfo(config Config, projectId string) (ProjectInfo, error) {
	infoPath := filepath.Join(config.ProjectDir, projectId, "info.json")
	infoFile, err := os.Open(infoPath)
	if err != nil {
		return ProjectInfo{}, fmt.Errorf("ReadProjectInfo: %w", err)
	}
	defer infoFile.Close()

	var info ProjectInfo
	err = json.NewDecoder(infoFile).Decode(&info)
	if err != nil {
		return ProjectInfo{}, fmt.Errorf("ReadProjectInfo: %w", err)
	}

	return info, nil
}

// WriteProjectInfo writes ProjectInfo for a project.
func WriteProjectInfo(config Config, projectId string, projectInfo ProjectInfo) error {
	infoPath := filepath.Join(config.ProjectDir, projectId, "info.json")
	infoFile, err := os.Create(infoPath)
	if err != nil {
		return fmt.Errorf("WriteProjectInfo: %w", err)
	}
	defer infoFile.Close()

	encoder := json.NewEncoder(infoFile)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(projectInfo)
	if err != nil {
		return fmt.Errorf("WriteProjectInfo: %w", err)
	}

	return nil
}

// NewProject creates a new project and gives it a random ID belonging
// to owner. It returns the ID.
func NewProject(config Config, owner string) (string, error) {
	tries := 0
	var id string

	for {
		var randBytes [ProjectIdByteLength]byte
		_, err := rand.Read(randBytes[:])
		if err != nil {
			return "", fmt.Errorf("NewProject random bytes: %w", err)
		}
		id = fmt.Sprintf("%x", randBytes)

		fullPath := filepath.Join(config.ProjectDir, id)
		if _, err := os.Stat(fullPath); err == nil {
			// Somehow random id already exists
			tries++
			if tries > 16 {
				return "", errors.New("NewProject randomness is broken")
			}
			continue
		}

		break
	}

	projectPath := filepath.Join(config.ProjectDir, id)
	if err := os.Mkdir(projectPath, os.ModePerm); err != nil {
		return "", fmt.Errorf("NewProject Mkdir: %w", err)
	}

	for _, subdir := range([]string{"aux", "out", "src"}) {
		subDirPath := filepath.Join(projectPath, subdir)
		if err := os.Mkdir(subDirPath, os.ModePerm); err != nil {
			return "", fmt.Errorf("NewProject subdir: %w", err)
		}
	}

	info := ProjectInfo{
		Owner: owner,
		CreatedAt: time.Now().UTC(),
	}

	if err := WriteProjectInfo(config, id, info); err != nil {
		return "", fmt.Errorf("NewProject: %w", err)
	}

	return id, nil
}

type FileInfo struct {
	Path string        `json:"path"`
	Size uint64        `json:"size"`
	Sha512Hash string  `json:"sha512hash"`
}

// ListProjectFiles returns a list of files in the subdir of a project
// directory.
//
// It will cache the list after creating it because hashing an unknown
// and potentially large number of files can be expensive. It will
// read from that cahe if it exists. The subdir cache should be
// deleted by any function that modifies the files it contains.
func ListProjectFiles(config Config, projectId string, subdir string) ([]FileInfo, error) {
	projectPath := filepath.Join(config.ProjectDir, projectId)
	filesPath := filepath.Join(projectPath, subdir)
	cachePath := filepath.Join(projectPath, fmt.Sprintf("%s_cahce.json", subdir))

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, errors.New("Project doesn't exist")
	}
	if _, err := os.Stat(filesPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Project %s directory doesn't exist", subdir)
	}

	var fileInfo []FileInfo

	// If cache exists, use it. It should be deleted if anything has changed
	if _, err := os.Stat(cachePath); err == nil {
		cacheFile, err := os.Open(cachePath)
		if err != nil {
			return nil, fmt.Errorf("ListProjectFiles opening cache: %w", err)
		}
		defer cacheFile.Close()

		err = json.NewDecoder(cacheFile).Decode(&fileInfo)
		if err != nil {
			return nil, fmt.Errorf("ListProjectFiles reading cache: %w", err)
		}

		return fileInfo, nil
	}

	filepath.Walk(filesPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			log.Printf("ListProjectFiles of \"%s\", path \"%s\": %s", filesPath, path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}

		size := info.Size()
		fileData, err := ioutil.ReadFile(path)
		hash := sha512.Sum512(fileData)
		digest := fmt.Sprintf("%x", hash)
		partialPath := strings.TrimPrefix(path, filesPath)

		fileInfo = append(fileInfo, FileInfo{ Path: partialPath, Sha512Hash: digest, Size: uint64(size) })

		return nil
	})

	cacheFile, err := os.Create(cachePath)
	if err != nil {
		return nil, fmt.Errorf("ListProjectFiles creating cache: %w", err)
	}
	defer cacheFile.Close()

	if err := json.NewEncoder(cacheFile).Encode(fileInfo); err != nil {
		return nil, fmt.Errorf("ListProjectFiles: encoding cache %w", err)
	}

	return fileInfo, nil
}

// ClearProjectDir empties a project's subdirectory. This would
// usually be something like src, aux, or out.
func ClearProjectDir(config Config, projectId string, subdir string) error {
	projectPath := filepath.Join(config.ProjectDir, projectId)
	subdirPath := filepath.Join(projectPath, subdir)
	cachePath := filepath.Join(projectPath, fmt.Sprintf("%scache.json", subdir))

	// If we try to remove and get the error that the file doesn't
	// exist, that's fine
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ClearProjectDir deleting cache: %w", err)
	}

	subdirFile, err := os.Open(subdirPath)
    if err != nil {
        return fmt.Errorf("ClearProjectDir: %w", err)
    }
    defer subdirFile.Close()

    names, err := subdirFile.Readdirnames(-1)
    if err != nil {
        return fmt.Errorf("CleanProjectDir: %w", err)
    }

    for _, name := range names {
        err = os.RemoveAll(filepath.Join(subdirPath, name))
        if err != nil {
            return fmt.Errorf("ClearProjectDir: %w", err)
        }
    }

	return nil
}

// Options for BuildProject
type ProjectBuildOptions struct {
	Force bool // Run latex in nonstop mode, and latexmk with force flag
	FileLineError bool // Erorrs are in c-style file:line:error format
	Engine Engine // LaTeX engine to use
	Document string // The name of the main document
}

// BuildProject builds a project using latexmk using the options
// provided. It retuens the stdout of latexmk.
func BuildProject(ctx context.Context, config Config, projectId string, options ProjectBuildOptions) (string, error) {
	projectPath := filepath.Join(config.ProjectDir, projectId)
	srcPath := filepath.Join(projectPath, "src")
	outPath := filepath.Join(projectPath, "out")
	auxPath := filepath.Join(projectPath, "aux")

	projectInfo, err := ReadProjectInfo(config, projectId)
	if err != nil {
		return "", fmt.Errorf("BuildProject: %w", err)
	}

	beginTime := time.Now()

	projectInfo.LastBuild = beginTime.UTC()
	projectInfo.BuildStatus = "running"
	projectInfo.BuildTime = ""

	if err := WriteProjectInfo(config, projectId, projectInfo); err != nil {
		return "", fmt.Errorf("BuildProject before build: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, config.MaxProjectBuildTime)

	buildOut, err := RunBuild(timeoutCtx, BuildOptions{
		AuxDir: auxPath,
		OutDir: outPath,
		SrcDir: srcPath,
		// SharedDir: "", // TODO Make shared dir work
		Document: options.Document,
		Engine: options.Engine,
		Force: options.Force,
		FileLineError: options.FileLineError,
	})
	buildTime := time.Since(beginTime)
	cancel() // Don't leak the context

	projectInfo, err = ReadProjectInfo(config, projectId)
	if err != nil {
		return "", fmt.Errorf("BuildProject: %w", err)
	}

	projectInfo.BuildStatus = "finished"
	projectInfo.BuildTime = buildTime.String()

	if err != nil {
		projectInfo.BuildStatus = "failed"
		if err := WriteProjectInfo(config, projectId, projectInfo); err != nil {
			return "", fmt.Errorf("BuildProject after failed build: %w", err)
		}
		return "", fmt.Errorf("BuildProject: %w", err)
	}

	if err := WriteProjectInfo(config, projectId, projectInfo); err != nil {
		return "", fmt.Errorf("BuildProject after build: %w", err)
	}

	return buildOut, nil
}

func DeleteProject(config Config, projectId string) error {
	projectPath := filepath.Join(config.ProjectDir, projectId)
	err := os.RemoveAll(projectPath)
	if err != nil {
		return fmt.Errorf("DeleteProject: %w", err)
	}
	return nil
}
