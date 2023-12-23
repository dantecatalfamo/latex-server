package main

import (
	"context"
	"log"
	"os"
	"time"
)

func main() {
	log.Println("Pulling image")
	ctx := context.Background()
	if err := PullImage(ctx); err != nil {
		panic(err)
	}
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
}
