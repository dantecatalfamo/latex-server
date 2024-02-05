package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dantecatalfamo/remotex/pkg/server"
)

const listenAddress = "localhost:3344"
const defaultConfigPath = "/etc/remotex/remotex.json"

func main() {
	configPath := flag.String("config", defaultConfigPath, "Configutation file")
	flag.Parse()

	cmd := flag.Args()
	if len(cmd) == 0 {
		usage()
		return
	}

	if cmd[0] == "newconfig" {
		if len(cmd) != 2 {
			fmt.Println("usage: remotex newconfig <path>")
			os.Exit(1)
		}
		if err := server.WriteNewConfig(cmd[1]); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return
	}

	if *configPath == "" {
		fmt.Println("Specify config path")
		os.Exit(1)
	}

	config, err := server.ReadAndInitializeConfig(*configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	log.Printf("Server config: %+v", config)

	switch cmd[0] {
	case "server":
		log.Printf("Listening on http://%s", config.ListenAddress)
		server.RunServer(config)
		if err != nil {
			log.Panic(err)
		}
	case "useradd":
		if len(cmd) < 2 {
			fmt.Println("usage: remotex useradd <username>")
			return
		}
		user := cmd[1]
		if err := server.CreateUser(config, user); err != nil {
			log.Fatal(err)
		}
		log.Printf("Added user %s", user)
	case "userdel":
		if len(cmd) < 2 {
			fmt.Println("usage: remotex userdel <username>")
			return
		}
		user := cmd[1]
		if err := server.DeleteUser(config, user); err != nil {
			log.Fatal(err)
		}
		log.Printf("Deleted user %s", user)
	case "tokenadd":
		if len(cmd) < 3 {
			fmt.Println("usage: remotex tokenadd <username> <description>")
			os.Exit(1)
		}
		user := cmd[1]
		desc := cmd[2]
		token, err := server.CreateUserToken(config, user, desc)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Token: %s\n", token)
	case "tokendel":
		if len(cmd) < 2 {
			fmt.Println("usage: remotex tokendel <token>")
			return
		}
		token := cmd[1]
		if err := server.DeleteUserToken(config, token); err != nil {
			log.Fatal(err)
		}
		log.Println("Token deleted")
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage: remotex-server [options] <command> [args]")
	flag.PrintDefaults()
	fmt.Printf(`
  commands:
    newconfig <file>
    server
    useradd   <username>
    userdel   <username>
    tokenadd  <username>
    tokendel  <token>
`)
}
