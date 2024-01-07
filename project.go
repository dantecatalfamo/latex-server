package main

import (
	"crypto/rand"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"
)

// We should be using a database like SQLite3 for this at some point,
// but for now we're just playing around
type ProjectInfo struct {
	Name string         `json:"name"`
	Owner string        `json:"owner"`
	CreatedAt time.Time `json:"createdAt"`
	LastBuild time.Time `json:"lastBuild"`
}

func ReadProjectInfo(config Config, projectId string) (ProjectInfo, error) {
	infoPath := filepath.Join(config.ProjectDir, projectId, "info.json")
	infoFile, err := os.Open(infoPath)
	if err != nil {
		return ProjectInfo{}, fmt.Errorf("ReadProjectInfo: %w", err)
	}

	var info ProjectInfo
	err = json.NewDecoder(infoFile).Decode(&info)
	if err != nil {
		return ProjectInfo{}, fmt.Errorf("ReadProjectInfo: %w", err)
	}

	return info, nil
}

func WriteProjectInfo(config Config, projectId string, projectInfo ProjectInfo) error {
	infoPath := filepath.Join(config.ProjectDir, projectId, "info.json")
	infoFile, err := os.Create(infoPath)
	if err != nil {
		return fmt.Errorf("WriteProjectInfo: %w", err)
	}

	encoder := json.NewEncoder(infoFile)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(projectInfo)
	if err != nil {
		return fmt.Errorf("WriteProjectInfo: %w", err)
	}

	return nil
}

func NewProject(config Config, owner string) (string, error) {
	tries := 0
	var id string

	for {
		var randBytes [24]byte
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

func ListProjectFiles(config Config, project string, subdir string) ([]FileInfo, error) {
	projectPath := filepath.Join(config.ProjectDir, project)
	filesPath := filepath.Join(projectPath, subdir)
	cachePath := filepath.Join(projectPath, fmt.Sprintf("%scahce.json", subdir))

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
		size := info.Size()

		fileData, err := fs.ReadFile(nil, path)
		hash := sha512.Sum512(fileData)
		digest := fmt.Sprintf("%x", hash)

		fileInfo = append(fileInfo, FileInfo{ Path: path, Sha512Hash: digest, Size: uint64(size) })

		return nil
	})

	cacheFile, err := os.Create(cachePath)
	if err != nil {
		return nil, fmt.Errorf("ListProjectFiles creating cache: %w", err)
	}
	if err := json.NewEncoder(cacheFile).Encode(fileInfo); err != nil {
		return nil, fmt.Errorf("ListProjectFiles: encoding cache %w", err)
	}

	return fileInfo, nil
}
