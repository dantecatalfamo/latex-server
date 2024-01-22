package main

import (
	"flag"
	"log"
	"time"

	"github.com/dantecatalfamo/remotex/pkg/server"
)

const listenAddress = "localhost:3344"

func main() {
	databasePath := flag.String("db", "latexServer.db", "Database path")
	projectsDir := flag.String("root", "latex", "Root of project directories")
	listenAddr := flag.String("listen", listenAddress, "Listen address")
	flag.Parse()

	db, err := server.NewDatabse(*databasePath)
	if err != nil {
		log.Fatalf("Failed to open database: %s", err)
	}

	config := server.Config{
		ProjectDir: *projectsDir,
		MaxProjectBuildTime: 30 * time.Second,
		Database: db,
		MaxFileSize: 25 * 1024 * 1024,
		ListenAddress: *listenAddr,
		BuildMode: server.BuildModeNative,
	}
	log.Printf("ProjectsDir: %s, Max Build Time: %s, Database: %v", config.ProjectDir, config.MaxProjectBuildTime, config.Database)

	cmd := flag.Args()
	if len(cmd) == 0 {
		flag.Usage()
		return
	}
	switch cmd[0] {
	case "server":
		log.Printf("Listening on http://%s", config.ListenAddress)
		server.RunServer(config)
		if err != nil {
			log.Panic(err)
		}
	case "useradd":
		if len(cmd) < 2 {
			return
		}
		user := cmd[1]
		if err := server.CreateUser(config, user); err != nil {
			log.Fatal(err)
		}
		log.Printf("Added user %s", user)
	case "userdel":
		if len(cmd) < 2 {
			return
		}
		user := cmd[1]
		if err := server.DeleteUser(config, user); err != nil {
			log.Fatal(err)
		}
		log.Printf("Deleted user %s", user)
	}
}
