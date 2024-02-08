package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dantecatalfamo/remotex/pkg/client"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	globalConfig, err := client.ReadGlobalConfig()
	if err != nil && !errors.Is(err, os.ErrNotExist){
		fmt.Println(err)
		os.Exit(1)
	}

	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := client.WriteGlobalConfig(client.GlobalConfig{}); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Wrote new empty global config")
		return
	}

	cmd := os.Args[1:]

	if cmd[0] == "global" {
		if len(cmd) < 2 {
			fmt.Println("user         ", globalConfig.User)
			fmt.Println("token        ", globalConfig.Token)
			fmt.Println("serverBaseUrl", globalConfig.ServerBaseUrl)
			return
		}
		saveConfig := false
		switch cmd[1] {
		case "serverBaseUrl":
			if len(cmd) == 2 {
				fmt.Println(globalConfig.ServerBaseUrl)
			} else if len(cmd) == 3 {
				globalConfig.ServerBaseUrl = cmd[2]
				saveConfig = true
			} else {
				fmt.Println("Too many arguments")
				os.Exit(1)
			}
		case "user":
			if len(cmd) == 2 {
				fmt.Println(globalConfig.User)
			} else if len(cmd) == 3 {
				globalConfig.User = cmd[2]
				saveConfig = true
			} else {
				fmt.Println("Too many arguments")
				os.Exit(1)
			}
		case "token":
			if len(cmd) == 2 {
				fmt.Println(globalConfig.Token)
			} else if len(cmd) == 3 {
				globalConfig.Token = cmd[2]
				saveConfig = true
			} else {
				fmt.Println("Too many arguments")
				os.Exit(1)
			}
		}
		if saveConfig {
			if err := client.WriteGlobalConfig(globalConfig); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		return
	}

	if cmd[0] == "login" {
		// TODO check if logged in, ask to logout before logging in
		if globalConfig.ServerBaseUrl == "" {
			fmt.Print("Server base url: ")
			reader := bufio.NewReader(os.Stdin)
			line, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			globalConfig.ServerBaseUrl = strings.Trim(line, "\n")
		}
		fmt.Printf("Login for %s\n", globalConfig.ServerBaseUrl)
		var username string
		fmt.Print("Username: ")
		fmt.Scanln(&username)
		fmt.Print("Password: ")
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println()

		if err := client.Login(globalConfig, username, string(password)); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println("Logged in")
		return
	}

	validateGlobalConfig(globalConfig)

	switch cmd[0] {
	case "init":
		if len(cmd) < 2 {
			fmt.Println("No project name")
			os.Exit(1)
		}

		projectName := cmd[1]
		path := projectName

		if len(cmd) > 2 {
			path = cmd[2]
		}

		if projectRoot, err := client.FindProjectRoot(); err == nil {
			fmt.Printf("Already in a project: %s", projectRoot)
			os.Exit(1)
		}

		if _, err := client.ReadProjectConfig(path); err == nil {
			fmt.Printf("Project \"%s\" already exists on remote", projectName)
			os.Exit(1)
		}

		ctx := context.Background()
 		if err := client.NewProject(ctx, globalConfig, projectName, path); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "files":
		projectRoot := findRoot()
		for _, subdir := range []string{"src", "aux", "out"} {
			files, err := client.ScanProjectFiles(projectRoot, subdir)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			fmt.Printf("%s:\n", subdir)
			for _, file := range files {
				fmt.Printf("  %+v\n", file)
			}
		}
	case "filesremote":
		projectRoot := findRoot()
		projectConfig := readProjectConfig(projectRoot)
		ctx := context.Background()

		for _, subdir := range []string{"src", "aux", "out"} {
			files, err := client.FetchProjectFileList(ctx, globalConfig, projectConfig.ProjectName, subdir)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			fmt.Printf("%s:\n", subdir)
			for _, file := range files {
				fmt.Printf("  %+v\n", file)
			}
		}
	case "project":
		projectRoot := findRoot()
		projectConfig := readProjectConfig(projectRoot)
		fmt.Println("projectName    ", projectConfig.ProjectName)
		fmt.Println("saveAuxFiles   ", projectConfig.SaveAuxFiles)
		fmt.Println("buildOptions")
		fmt.Println("  force        ", projectConfig.BuildOptions.Force)
		fmt.Println("  fileLineError", projectConfig.BuildOptions.FileLineError)
		fmt.Println("  engine       ", projectConfig.BuildOptions.Engine)
		fmt.Println("  document     ", projectConfig.BuildOptions.Document)
		fmt.Println("  dependents   ", projectConfig.BuildOptions.Dependents)
		fmt.Println("  cleanBuild   ", projectConfig.BuildOptions.CleanBuild)
	case "info":
		projectRoot := findRoot()
		projectConfig := readProjectConfig(projectRoot)
		ctx := context.Background()

		info, err := client.FetchProjectInfo(ctx, globalConfig, projectConfig.ProjectName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("name", info.Name)
		fmt.Println("createdAt", info.CreatedAt)
		fmt.Println("public", info.Public)
		fmt.Println("lastBuildStart", info.LatestBuild.BuildStart)
		fmt.Println("lastBuildTime", info.LatestBuild.BuildTime)
		fmt.Println("lastBuildStatus", info.LatestBuild.Status)
		fmt.Println("lastBuildOptions")
		fmt.Println("  force        ", info.LatestBuild.Options.Force)
		fmt.Println("  fileLineError", info.LatestBuild.Options.FileLineError)
		fmt.Println("  engine       ", info.LatestBuild.Options.Engine)
		fmt.Println("  document     ", info.LatestBuild.Options.Document)
		fmt.Println("  dependents   ", info.LatestBuild.Options.Dependents)
		fmt.Println("  cleanBuild   ", info.LatestBuild.Options.CleanBuild)
		fmt.Println("buildOut")
		fmt.Print(info.LatestBuild.BuildOut)
	case "build":
		projectRoot := findRoot()
		projectConfig := readProjectConfig(projectRoot)
		ctx := context.Background()
		buildOut, err := client.BuildAndSyncProject(ctx, globalConfig, projectConfig, projectRoot)
		if err != nil {
			fmt.Println("Error:", err)
		}
		fmt.Print(buildOut)
	case "pull":
		projectRoot := findRoot()
		projectConfig := readProjectConfig(projectRoot)
		ctx := context.Background()
		if err := client.PullAllProjectFiles(ctx, globalConfig, projectConfig, projectRoot); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "user":
		ctx := context.Background()
		userInfo, err := client.FetchUserInfo(ctx, globalConfig)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("%+v\n", userInfo)
	case "listprojects":
		ctx := context.Background()
		userInfo, err := client.FetchUserInfo(ctx, globalConfig)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		for _, project := range userInfo.Projects {
			fmt.Printf("- %s\n  public: %v,\n  build: %s\n", project.Name, project.Public, project.LatestBuild.Status)
		}
	case "clone":
		if len(cmd) < 2 {
			fmt.Println("No project name")
			os.Exit(1)
		}
		projectName := cmd[1]
		path := projectName
		if len(cmd) > 2 {
			path = cmd[2]
		}
		ctx := context.Background()
		if err := client.CloneProject(ctx, globalConfig, projectName, path); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Println("Invalid command")
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`Usage: remotex <command> [args]
commands:
  login        Login to remotex server
  build        Build the current project
  clone        Clone an existing project to your local machien
  files        List the current project's local files
  filesremote  List the current project's remote files
  global       Read or write global config
  init         Create a new project
  listprojects List all remote projects
  project      Read or write project config
  pull         Pull any missing files from project remote
  user         Read user info from remote
`)
}

func findRoot() string {
	projectRoot, err := client.FindProjectRoot()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return projectRoot
}

func readProjectConfig(root string) client.ProjectConfig {
	config, err := client.ReadProjectConfig(root)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return config
}

func validateGlobalConfig(globalConfig client.GlobalConfig) {
	if globalConfig.ServerBaseUrl == "" {
		fmt.Println("Error: config serverBaseUrl empty")
		os.Exit(1)
	}

	if globalConfig.User == "" {
		fmt.Println("Error: config user empty")
		os.Exit(1)
	}

	if globalConfig.Token == "" {
		fmt.Println("Error: config token empty")
		os.Exit(1)
	}
}
