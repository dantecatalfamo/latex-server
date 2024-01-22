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
		projectName := cmd[1]
		path := cmd[2]
		if err := client.NewProject(globalConfig, projectName, path); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "list":
		projectRoot, err := client.FindProjectRoot()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
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
	default:
		fmt.Println("Invalid command")
		os.Exit(1)
	}
}
