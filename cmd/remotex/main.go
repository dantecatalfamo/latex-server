package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/dantecatalfamo/remotex/pkg/client"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("No command")
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
			fmt.Printf("%+v\n", globalConfig)
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

	validateGlobalConfig(globalConfig)

	switch cmd[0] {
	case "init":
		if len(cmd) < 2 {
			fmt.Println("No project name")
			os.Exit(1)
		}

		projectName := cmd[1]
		var path string

		if len(cmd) < 3 {
			path = projectName
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
		for _, subdir := range []string{"src", "aux", "out"} {
			files, err := client.FetchProjectFileList(globalConfig, projectConfig.ProjectName, subdir)
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
		fmt.Printf("%+v\n", projectConfig)
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
	default:
		fmt.Println("Invalid command")
		os.Exit(1)
	}
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
