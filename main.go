package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type BuildOptions struct {
	Context context.Context
	AuxDir string
	OutDir string
	TexDir string
	Document string
}

const TexLiveContainer = "texlive/texlive:latest"

func RunBuild(options BuildOptions) error {
	if options.TexDir == "" {
		return errors.New("TexDir empty")
	}
	if options.OutDir == "" {
		return errors.New("OutDir empty")
	}

	ctx := options.Context

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("RunBuild NewClientWithOpts: %w", err)
	}
	defer cli.Close()

	reader, err := cli.ImagePull(ctx, TexLiveContainer, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("RunBuild ImagePull: %w", err)
	}

	defer reader.Close()
	// cli.ImagePull is asynchronous.
	// The reader needs to be read completely for the pull operation to complete.
	// If stdout is not required, consider using io.Discard instead of os.Stdout.

	if _, err := io.Copy(os.Stdout, reader); err != nil {
		return fmt.Errorf("RunBuild ImagePull reader copy: %w", err)
	}

	var mountAux mount.Mount
	if options.AuxDir == "" {
		mountAux = mount.Mount{
			Type: mount.TypeTmpfs,
			Target: "/mnt/aux",
		}
	} else {
		mountAux = mount.Mount{
			Type: mount.TypeBind,
			Target: "/mnt/aux",
			Source: options.AuxDir,
		}
	}

	resp, err := cli.ContainerCreate(
		ctx,
		&container.Config{
			Image: TexLiveContainer,
			Cmd:   []string{"latexmk", "-pdf", "-silent", "-auxdir=/mnt/aux", "-outdir=/mnt/out"},
			Tty:   false,
			NetworkDisabled: true,
		},
		&container.HostConfig{
			AutoRemove: true,
			Mounts: []mount.Mount{
				{
					Type: mount.TypeBind,
					Source: options.TexDir,
					Target: "/workdir",
				},
				{
					Type: mount.TypeBind,
					Target: "/mnt/out",
					Source: options.OutDir,
				},
				mountAux,
			},
		}, nil, nil, "",
	)
	if err != nil {
		return fmt.Errorf("RunBuild ContainerCreate: %w", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("RunBuild ContainerStart: %w", err)
	}

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return fmt.Errorf("RunBuild ContainerLogs: %w", err)
	}

	log.Println("container logs")

	if _, err := stdcopy.StdCopy(os.Stdout, os.Stderr, out); err != nil {
		return fmt.Errorf("RunBuild ContainerLogs StdCopy: %w", err)
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("RunBuild ContainerWait: %w", err)
		}
	case status := <-statusCh:
		if status.Error != nil {
			return fmt.Errorf("RunBuild ContainerWait status: %w", err)
		}
		if status.StatusCode != 0 {
			return fmt.Errorf("RunBuild ContainerWait non-zero exit: %d", status.StatusCode)
		}
	}

	return nil
}

func main() {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	log.Println(cwd)
	if err := RunBuild(BuildOptions{ TexDir: cwd, OutDir: "/tmp/latex", Context: ctx }); err != nil {
		panic(err)
	}
}
