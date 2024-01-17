package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

const listenAddress = "localhost:3344"

type Config struct {
	ProjectDir string // Root of all projects
	MaxProjectBuildTime time.Duration // Max time a project can build
}

func main() {
	if len(os.Args) < 2 {
		usage();
	}
	command := os.Args[1]
	switch command {
	case "server":
		if len(os.Args) < 3 { usage() }
		projectsDir := os.Args[2]
		config := Config{
			ProjectDir: projectsDir,
			MaxProjectBuildTime: 30 * time.Second,
		}
		log.Printf("ProjectsDir: %s, Max Build Time: %s", config.ProjectDir, config.MaxProjectBuildTime)
		mux := chi.NewMux()
		SetupRoutes(config, mux)
		log.Printf("Listening on http://%s", listenAddress)
		err := http.ListenAndServe(listenAddress, mux)
		if err != nil {
			log.Panic(err)
		}
	case "pull":
		log.Println("Pulling image")
		ctx := context.Background()
		if err := PullImage(ctx); err != nil {
			panic(err)
		}
	case "testbuild":
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		log.Println(cwd)

		ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
		out, err := RunBuild(ctx, BuildOptions{ SrcDir: cwd, OutDir: cwd })
		if err != nil {
			panic(err)
		}
		cancel()
		log.Print(out)
	case "db":
		db, err := NewDatabse("/tmp/ass.db")
		fmt.Printf("db: %v, err: %v\n", db, err)
	default:
		usage()
	}
}

func usage() {
	fmt.Printf("usage: latex-server <command>\n  server <projects root>: Run server\n  pull: Pull latest image\n")
	os.Exit(1)
}
