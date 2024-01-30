package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dantecatalfamo/remotex/pkg/server"
)

const ProjectConfigName = ".remotex"

func NewProject(ctx context.Context, globalConfig GlobalConfig, projectName, projectRoot string) error {
	// Should fetch project info before continuing
	_, fetchErr := FetchProjectInfo(ctx, globalConfig, projectName)
	if fetchErr == nil {
		return ErrProjectExists
	}

	if !errors.Is(fetchErr, ErrProjectNotExist) {
		return fmt.Errorf("NewProject info fetch: %w", fetchErr)
	}

	// Project does not exist

	if err := CreateRemoteProject(ctx, globalConfig, projectName); err != nil {
		return fmt.Errorf("NewProject create remote project: %w", err)
	}

	if err := os.MkdirAll(projectRoot, 0700); err != nil {
		return fmt.Errorf("NewProject MkdirAll: %w", err)
	}

	for _, subdir := range []string{"aux", "out", "src"} {
		subdirPath := filepath.Join(projectRoot, subdir)
		os.Mkdir(subdirPath, 0700)
	}

	projectConfig := ProjectConfig{ProjectName: projectName}

	if err := WriteProjectConfig(projectRoot, projectConfig); err != nil {
		return fmt.Errorf("NewProject write config: %w", err)
	}

	return nil
}

func CreateRemoteProject(ctx context.Context, globalConfig GlobalConfig, projectName string) error {
	userUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, globalConfig.User)
	if err != nil {
		return fmt.Errorf("CreateRemoteProject join url: %w", err)
	}

	form := url.Values{}
	form["project"] = []string{projectName}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, userUrl, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", globalConfig.Token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("CreateRemoteProject do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("CreateRemoteProject unexpected status code %d", resp.StatusCode)
	}

	return nil
}

func FetchProjectInfo(ctx context.Context, globalConfig GlobalConfig, projectName string) (server.ProjectInfo, error) {
	projectUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, globalConfig.User, projectName)
	if err != nil {
		return server.ProjectInfo{}, fmt.Errorf("FetchProjectInfo path join: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, projectUrl, nil)
	if err != nil {
		return server.ProjectInfo{}, fmt.Errorf("FetchProjectInfo create request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", globalConfig.Token))

	resp, err := http.DefaultClient.Do(req)
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
			if entry.Name() == ProjectConfigName {
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

func FetchProjectFileList(ctx context.Context, globalConfig GlobalConfig, projectName, subdir string) ([]server.FileInfo, error) {
	filesUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, globalConfig.User, projectName, subdir)
	if err != nil {
		return nil, fmt.Errorf("FetchProjectFileList join path: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, filesUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("FetchProjectFileList create request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", globalConfig.Token))

	resp, err := http.DefaultClient.Do(req)
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

func PullProjectFile(ctx context.Context, globalConfig GlobalConfig, projectConfig ProjectConfig, projectRoot, subdir, filePath string) (int64, error) {
	fileUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, globalConfig.User, projectConfig.ProjectName, subdir, filePath)
	if err != nil {
		return 0, fmt.Errorf("PullProjectFile join url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileUrl, nil)
	if err != nil {
		return 0, fmt.Errorf("PullProjectFile create request object: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", globalConfig.Token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("PullProjectFile do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("PullProjectFile unexpected error code %d", resp.StatusCode)
	}

	localPath := filepath.Join(projectRoot, subdir, filePath)
	file, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("PullProjectFile create file: %w", err)
	}
	defer file.Close()

	size, err := io.Copy(file, resp.Body)
	if err != nil {
		return 0, fmt.Errorf("PullProjectFile write file: %w", err)
	}

	return size, nil
}

func PushProjectFile(ctx context.Context, globalConfig GlobalConfig, projectConfig ProjectConfig, projectRoot, subdir, filePath string) (int64, error) {
	subdirUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, globalConfig.User, projectConfig.ProjectName, subdir)
	if err != nil {
		return 0, fmt.Errorf("PushProjectFile join url: %w", err)
	}

	localPath := filepath.Join(projectRoot, subdir, filePath)
	file, err := os.Open(localPath)
	if err != nil {
		return 0, fmt.Errorf("PushProjectFile open file: %w", err)
	}
	defer file.Close()

	body := new(bytes.Buffer)
	form := multipart.NewWriter(body)
	part, err := form.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return 0, fmt.Errorf("PushProjectFile create form file: %w", err)
	}
	size, err := io.Copy(part, file)
	if err != nil {
		return 0, fmt.Errorf("PushProjectFile write form file: %w", err)
	}
	pathWriter, err := form.CreateFormField("path")
	if err != nil {
		return 0, fmt.Errorf("PushProjectFile create form path: %w", err)
	}
	if _, err := fmt.Fprint(pathWriter, filePath); err != nil {
		return 0, fmt.Errorf("PushProjectFile write form path: %w", err)
	}
	form.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, subdirUrl, body)
	req.Header.Add("Content-Type", form.FormDataContentType())
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", globalConfig.Token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("PushProjectFile send post request: %w", err)
	}

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("PushProjectFile unexpected status code %d", resp.StatusCode)
	}

	return size, nil
}

func DeleteRemoteProjectFile(ctx context.Context, globalConfig GlobalConfig, projectConfig ProjectConfig, subdir, filePath string) error {
	fileUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, globalConfig.User, projectConfig.ProjectName, subdir, filePath)
	if err != nil {
		return fmt.Errorf("DeleteRemoteProjectFile join url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fileUrl, nil)
	if err != nil {
		return fmt.Errorf("DeleteRemoteProjectFile create request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", globalConfig.Token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("DeleteRemoteProjectFile do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func DeleteLocalProjectFile(projectRoot, subdir, filePath string) error {
	fullPath := filepath.Join(projectRoot, subdir, filePath)

	if fullPath == "" || fullPath == "." || fullPath == "./" || fullPath == ".." {
		return errors.New("path is just the top level directory")
	}

	if strings.Contains(fullPath, "../") {
		return errors.New("path contains parent directory traversal")
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("DeleteLocalProjectFile stat: %w", err)
	}

	if stat.IsDir() {
		if err := os.RemoveAll(fullPath); err != nil {
			return fmt.Errorf("DeleteLocalProjectFile RemoveAll: %w", err)
		}
	} else {
		if err := os.Remove(fullPath); err != nil {
			return fmt.Errorf("DeleteLocalProjectFile Remove: %w", err)
		}
	}

	// Traverse dirs upward and delete any empty dirs until we get to
	// the top of the subdir, or we find a non-empty dir
	dirPath := filepath.Dir(fullPath)
	topDirPath := filepath.Join(projectRoot, subdir)
	for dirPath != topDirPath {
		empty, err := server.IsDirEmpty(dirPath)
		if err != nil {
			return fmt.Errorf("DeleteLocalProjectFile checking empty dir: %w", err)
		}

		if !empty {
			break
		}

		if err := os.Remove(dirPath); err != nil {
			return fmt.Errorf("DeleteLocalProjectFile clearing empty dirs: %w", err)
		}

		dirPath = filepath.Dir(dirPath)
	}

	return nil
}

func PushProjectFilesChanges(ctx context.Context, globalConfig GlobalConfig, projectConfig ProjectConfig, projectRoot, subdir string) error {
	localFiles, err := ScanProjectFiles(projectRoot, subdir)
	if err != nil {
		return fmt.Errorf("PushProjectFilesChanges scan local files: %w", err)
	}
	remoteFiles, err := FetchProjectFileList(ctx, globalConfig, projectConfig.ProjectName, subdir)
	if err != nil {
		return fmt.Errorf("PushProjectFilesChanges scan remote files: %w", err)
	}
	diff := DiffFileInfoLists(remoteFiles, localFiles)
	for _, deleted := range diff.Removed {
		if err := DeleteRemoteProjectFile(ctx, globalConfig, projectConfig, subdir, deleted.Path); err != nil {
			return fmt.Errorf("PushProjectFilesChanges delete remote file %s: %w", deleted.Path, err)
		}
	}
	for _, added := range diff.Added {
		if _, err := PushProjectFile(ctx, globalConfig, projectConfig, projectRoot, subdir, added.Path); err != nil {
			return fmt.Errorf("PushProjectFilesChanges upload file %s: %w", added.Path, err)
		}
	}

	return nil
}

func PullProjectFilesChanges(ctx context.Context, globalConfig GlobalConfig, projectConfig ProjectConfig, projectRoot, subdir string) error {
	localFiles, err := ScanProjectFiles(projectRoot, subdir)
	if err != nil {
		return fmt.Errorf("PullProjectFilesChanges scan local files: %w", err)
	}
	remoteFiles, err := FetchProjectFileList(ctx, globalConfig, projectConfig.ProjectName, subdir)
	if err != nil {
		return fmt.Errorf("PullProjectFilesChanges scan remote files: %w", err)
	}
	diff := DiffFileInfoLists(localFiles, remoteFiles)
	for _, deleted := range diff.Removed {
		if err := DeleteLocalProjectFile(projectRoot, subdir, deleted.Path); err != nil {
			return fmt.Errorf("PullProjectFilesChanges delete local file %s: %w", deleted.Path, err)
		}
	}
	for _, added := range diff.Added {
		if _, err := PullProjectFile(ctx, globalConfig, projectConfig, projectRoot, subdir, added.Path); err != nil {
			return fmt.Errorf("PullProjectFilesChanges pull remote file %s: %w", added.Path, err)
		}
	}

	return nil
}

type FileInfoMove struct {
	From server.FileInfo
	To server.FileInfo
}

type FileInfoDiff struct {
	Removed []server.FileInfo
	Added []server.FileInfo
	Same []server.FileInfo
}

func DiffFileInfoLists(original []server.FileInfo, other []server.FileInfo) FileInfoDiff {
	// TODO doesn't handle moved files, but neither does the server (for now)
	var removed []server.FileInfo
	var added []server.FileInfo
	var same []server.FileInfo

outerRemoved:
	for _, origFile := range original {
		for _, otherFile := range other {
			if origFile.Sha256Sum == otherFile.Sha256Sum && origFile.Path == otherFile.Path {
				same = append(same, origFile)
				continue outerRemoved
			}
		}
		removed = append(removed, origFile)
	}

outerAdded:
	for _, otherFile := range other {
		for _, origFile := range original {
			if origFile.Sha256Sum == otherFile.Sha256Sum && origFile.Path == otherFile.Path {
				continue outerAdded
			}
		}

		added = append(added, otherFile)
	}

	return FileInfoDiff{
		Removed: removed,
		Added: added,
		Same: same,
	}
}

func BuildProject(ctx context.Context, globalConfig GlobalConfig, projectConfig ProjectConfig) (string, error) {
	buildUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, globalConfig.User, projectConfig.ProjectName, "build")
	if err != nil {
		return "", fmt.Errorf("BuildProject join url: %w", err)
	}

	// XXX keep up to date with build options!

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildUrl, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", globalConfig.Token))
	query := req.URL.Query()

	if projectConfig.BuildOptions.CleanBuild {
		query.Add("cleanBuild", "true")
	}

	if projectConfig.BuildOptions.Dependents {
		query.Add("dependents", "true")
	}

	if projectConfig.BuildOptions.Document != "" {
		query.Add("document", projectConfig.BuildOptions.Document)
	}

	if projectConfig.BuildOptions.Engine != "" {
		query.Add("engine", string(projectConfig.BuildOptions.Engine))
	}

	if projectConfig.BuildOptions.FileLineError {
		query.Add("fileLineError", "true")
	}

	if projectConfig.BuildOptions.Force {
		query.Add("force", "true")
	}

	req.URL.RawQuery = query.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("BuildProject http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnprocessableEntity {
		// something wrong
		return "", fmt.Errorf("BuildProject unexpected http status code: %d", resp.StatusCode)
	}

	var outBuf bytes.Buffer
	if _, err := io.Copy(&outBuf, resp.Body); err != nil {
		return "", fmt.Errorf("BuildProject copy buffer: %w", err)
 	}

	if resp.StatusCode == http.StatusUnprocessableEntity {
		return outBuf.String(), ErrBuildFailure
	} else if resp.StatusCode == http.StatusConflict {
		return "", ErrBuildInProgress
	}

	return outBuf.String(), nil
}

var ErrBuildFailure = errors.New("build failure")
var ErrBuildInProgress = server.ErrBuildInProgress

func BuildAndSyncProject(ctx context.Context, globalConfig GlobalConfig, projectConfig ProjectConfig, projectRoot string) (string, error) {
	if err := PushProjectFilesChanges(ctx, globalConfig, projectConfig, projectRoot, "src"); err != nil {
		return "", fmt.Errorf("BuildAndSyncProject push src: %w", err)
	}

	buildOut, err := BuildProject(ctx, globalConfig, projectConfig)
	if err != nil {
		if errors.Is(err, ErrBuildFailure) {
			return buildOut, fmt.Errorf("BuildAndSyncProject build failure: %w", err)
		}
		return "", fmt.Errorf("BuildAndSyncProject build: %w", err)
	}

	if projectConfig.SaveAuxFiles {
		if err := PullProjectFilesChanges(ctx, globalConfig, projectConfig, projectRoot, "aux"); err != nil {
			return buildOut, fmt.Errorf("BuildAndSyncProject sync aux: %w", err)
		}
	}

	if err := PullProjectFilesChanges(ctx, globalConfig, projectConfig, projectRoot, "out"); err != nil {
		return buildOut, fmt.Errorf("BuildAndSyncPrject sync out: %w", err)
	}

	return buildOut, nil
}

func PullAllProjectFiles(ctx context.Context, globalConfig GlobalConfig, projectConfig ProjectConfig, projectRoot string) error {
	subdirs := []string{"src", "out"}
	if projectConfig.SaveAuxFiles {
		subdirs = append(subdirs, "aux")
	}

	for _, subdir := range subdirs {
		if err := PullProjectFilesChanges(ctx, globalConfig, projectConfig, projectRoot, subdir); err != nil {
			return fmt.Errorf("PullAllProjectFiles: %w", err)
		}
	}
	return nil
}

func CloneProject(ctx context.Context, globalConfig GlobalConfig, projectName, path string) error {
	projectInfo, err := FetchProjectInfo(ctx, globalConfig, projectName)
	if err != nil {
		return fmt.Errorf("CloneProject fetch project info: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("CloneProject unable to get cwd: %w", err)
	}
	projectRoot := filepath.Join(cwd, path)

	_, err = os.Stat(projectRoot)
	if err == nil {
		return fmt.Errorf("CloneProject directory already exists")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("CloneProject dir stat: %w", err)
	}

	if err := os.Mkdir(projectRoot, 0700); err != nil {
		return fmt.Errorf("CloneProject make project root: %w", err)
	}

	for _, subdir := range []string{"src", "aux", "out"} {
		subdirPath := filepath.Join(projectRoot, subdir)
		if err := os.Mkdir(subdirPath, 0700); err != nil {
			return fmt.Errorf("CloneProject make subdir: %w", err)
		}
	}

	projectConfig := ProjectConfig{}
	projectConfig.ProjectName = projectInfo.Name
	projectConfig.BuildOptions = projectInfo.LatestBuild.Options

	if err := WriteProjectConfig(projectRoot, projectConfig); err != nil {
		return fmt.Errorf("CloneProject write config: %w", err)
	}

	if err := PullAllProjectFiles(ctx, globalConfig, projectConfig, projectRoot); err != nil {
		return fmt.Errorf("CloneProject pull files: %w", err)
	}

	return nil
}

func FetchUserInfo(ctx context.Context, globalConfig GlobalConfig) (server.UserInfo, error) {
	userUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, globalConfig.User)
	if err != nil {
		return server.UserInfo{}, fmt.Errorf("FetchUserInfo path join: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userUrl, nil)
	if err != nil {
		return server.UserInfo{}, fmt.Errorf("FetchUserInfo create request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", globalConfig.Token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return server.UserInfo{}, fmt.Errorf("FetchUserInfo http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return server.UserInfo{}, fmt.Errorf("FetchUserInfo unexpected status code: %d", resp.StatusCode)
	}

	var userInfo server.UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return server.UserInfo{}, fmt.Errorf("FetchUserInfo decode: %w", err)
	}

	return userInfo, nil
}
