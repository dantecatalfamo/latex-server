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

type Config struct {
	ProjectDir string
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
		config := Config{ ProjectDir: projectsDir }
		mux := chi.NewMux()
		SetupRoutes(config, mux)
		err := http.ListenAndServe("localhost:3344", mux)
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
		out, err := RunBuild(ctx, BuildOptions{ TexDir: cwd, OutDir: cwd })
		if err != nil {
			panic(err)
		}
		cancel()
		log.Print(out)
	default:
		usage()
	}
}

func usage() {
	fmt.Printf("usage: latex-server <command>\n  server <projects root>: Run server\n  pull: Pull latest image\n")
	os.Exit(1)
}
