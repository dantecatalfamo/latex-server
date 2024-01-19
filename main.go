package main

import (
	"context"
	"flag"
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
	Database *Database // Database object
	MaxFileSize uint // Maximum upload size
}

func main() {
	databasePath := flag.String("db", "latexServer.db", "Database path")
	projectsDir := flag.String("root", "latex", "Root of project directories")
	listenAddr := flag.String("listen", listenAddress, "Listen address")
	flag.Parse()

	db, err := NewDatabse(*databasePath)
	if err != nil {
		log.Fatalf("Failed to open database: %s", err)
	}

	config := Config{
		ProjectDir: *projectsDir,
		MaxProjectBuildTime: 30 * time.Second,
		Database: db,
		MaxFileSize: 25 * 1024 * 1024,
	}
	log.Printf("ProjectsDir: %s, Max Build Time: %s, Database: %v", config.ProjectDir, config.MaxProjectBuildTime, config.Database)

	cmd := flag.Args()
	if len(cmd) == 0 {
		flag.Usage()
		return
	}
	switch cmd[0] {
	case "server":
		mux := chi.NewMux()
		SetupRoutes(config, mux)
		log.Printf("Listening on http://%s", listenAddress)
		err = http.ListenAndServe(*listenAddr, mux)
		if err != nil {
			log.Panic(err)
		}
	case "useradd":
		if len(cmd) < 2 {
			return
		}
		user := cmd[1]
		if err := CreateUser(config, user); err != nil {
			log.Fatal(err)
		}
		log.Printf("Added user %s", user)
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
	fmt.Printf("usage: latex-server <command>\n  server <projects root> <database file>\n    Run server\n  pull\n    Pull latest image\n")
	os.Exit(1)
}
