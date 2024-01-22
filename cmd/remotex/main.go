package main

import (
	"fmt"
	"os"

	"github.com/dantecatalfamo/remotex/pkg/client"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("No command")
		os.Exit(1)
	}

	globalConfig := client.GlobalConfig{
		User: "admin",
		ServerBaseUrl: "http://localhost:3344",
	}
	_ = globalConfig

	cmd := os.Args[1:]
	switch cmd[0] {
	case "init":
		if len(cmd) < 2 {
			fmt.Println("No project name")
			os.Exit(1)
		}
		if len(cmd) < 3 {
			fmt.Println("No project path")
			os.Exit(1)
		}

		if projectRoot, err := client.FindProjectRoot(); err == nil {
			fmt.Printf("Already in a project: %s", projectRoot)
			os.Exit(1)
		}

		projectName := cmd[1]
		path := cmd[2]

		if _, err := client.ReadProjectConfig(path); err == nil {
			fmt.Println("Project already exists")
			os.Exit(1)
		}

		if err := client.NewProject(globalConfig, projectName, path); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "list":
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
	case "listremote":
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
	case "config":
		projectRoot := findRoot()
		projectConfig, err := client.ReadProjectConfig(projectRoot)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("%+v\n", projectConfig)
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
