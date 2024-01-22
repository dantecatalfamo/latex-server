package client

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dantecatalfamo/remotex/pkg/server"
)

const RepoConfigName = ".remotex"

func NewProject(globalConfig GlobalConfig, projectName, path string) error {
	// Should fetch project info before continuing
	_, fetchErr := FetchProjectInfo(globalConfig, projectName)
	if fetchErr == nil {
		return ErrProjectExists
	}

	if !errors.Is(fetchErr, ErrProjectNotExist) {
		return fmt.Errorf("NewProject info fetch: %w", fetchErr)
	}

	// Project does not exist

	if err := os.MkdirAll(path, 0700); err != nil {
		return fmt.Errorf("NewProject MkdirAll: %w", err)
	}

	for _, subdir := range []string{"aux", "out", "src"} {
		subdirPath := filepath.Join(path, subdir)
		os.Mkdir(subdirPath, 0700)
	}

	repoConfig := RepoConfig{ProjectName: projectName}
	repoConfigPath := filepath.Join(path, RepoConfigName)

	file, err := os.Create(repoConfigPath)
	if err != nil {
		return fmt.Errorf("NewProject create repo config: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(repoConfig); err != nil {
		return fmt.Errorf("NewProject encode repo config: %w", err)
	}

	return nil
}

func FetchProjectInfo(globalConfig GlobalConfig, projectName string) (server.ProjectInfo, error) {
	// TODO incorporate auth at some point

	projectUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, globalConfig.User, projectName)
	if err != nil {
		return server.ProjectInfo{}, fmt.Errorf("FetchProjectInfo path join: %w", err)
	}

	resp, err := http.Get(projectUrl)
	if err != nil {
		return server.ProjectInfo{}, fmt.Errorf("FetchProjectInfo http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return server.ProjectInfo{}, ErrProjectNotExist
	}

	if resp.StatusCode != 200 {
		return server.ProjectInfo{}, fmt.Errorf("unecpected status code %d", resp.StatusCode)
	}

	var projectInfo server.ProjectInfo

	if err := json.NewDecoder(resp.Body).Decode(&projectInfo); err != nil {
		return server.ProjectInfo{}, fmt.Errorf("FetchProjectInfo decode json: %w", err)
	}

	return projectInfo, nil
}

var ErrProjectExists = errors.New("project already exists")
var ErrProjectNotExist = errors.New("project does not exist")

func FindProjectRoot() (string, error) {
	path, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("FindProjectRoot getwd: %w", err)
	}

	var pathTop string
	if runtime.GOOS == "windows" {
		pathTop = filepath.VolumeName(path)
	} else {
		pathTop = "/"
	}

	for path != pathTop {
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", fmt.Errorf("FindProjectRoot read dir: %w", err)
		}
		for _, entry := range entries {
			if entry.Name() == RepoConfigName {
				return path, nil
			}
		}

		path = filepath.Dir(path)
	}

	return "", ErrNoProjectRoot
}

var ErrNoProjectRoot = errors.New("no project root")

func ScanProjectFiles(projectRoot, subdir string) ([]server.FileInfo, error) {
	subdirPath := filepath.Join(projectRoot, subdir)
	removePrefix := subdirPath + string(filepath.Separator)

	var fileInfos []server.FileInfo

	filepath.Walk(subdirPath, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if err != nil {
			log.Printf("Scan error %s: %s", path, err)
			return nil
		}

		// Don't go into git directory if it exists
		if filepath.Base(path) == ".git" {
			return filepath.SkipDir
		}

		fileData, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Read file error: %s", err)
		}
		hash := sha256.Sum256(fileData)
		digest := fmt.Sprintf("%x", hash)
		partialPath := strings.TrimPrefix(path, removePrefix)

		fileInfo := server.FileInfo{
			Path: partialPath,
			Size: uint64(info.Size()),
			Sha256Sum: digest,
		}

		fileInfos = append(fileInfos, fileInfo)

		return nil
	})

	return fileInfos, nil
}

func FetchProjectFileList(globalConfig GlobalConfig, projectName, subdir string) ([]server.FileInfo, error) {
	// TODO add auth

	filesUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, globalConfig.User, projectName, subdir)
	if err != nil {
		return nil, fmt.Errorf("FetchProjectFileList join path: %w", err)
	}

	resp, err := http.Get(filesUrl)
	if err != nil {
		return nil, fmt.Errorf("FetchProjectFileList http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, ErrProjectNotExist
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("FetchProjectFileList unecpected status code %d", resp.StatusCode)
	}

	var fileInfos []server.FileInfo

	if err := json.NewDecoder(resp.Body).Decode(&fileInfos); err != nil {
		return nil, fmt.Errorf("FetchProjectFileList decode json: %w", err)
	}

	return fileInfos, nil
}
