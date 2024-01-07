package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type Engine string

const (
	EnginePDF Engine = "-pdf"
	EngineLua Engine = "-pdflua"
	EngineXeTeX Engine = "-pdfxe"
)

type BuildOptions struct {
	AuxDir string
	OutDir string
	TexDir string
	SharedDir string
	Document string
	Engine Engine
	Force bool
}

const TexLiveContainer = "texlive/texlive:latest"

// PullImage pulls the latest version of `TexLiveContainer'.
// It dumps the contents of the cli.ImagPull reader to stdout.
func PullImage(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("PullImage NewClientWithOpts: %w", err)
	}
	defer cli.Close()

	reader, err := cli.ImagePull(ctx, TexLiveContainer, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("PullImage ImagePull: %w", err)
	}

	defer reader.Close()
	// cli.ImagePull is asynchronous.
	// The reader needs to be read completely for the pull operation to complete.
	// If stdout is not required, consider using io.Discard instead of os.Stdout.

	if _, err := io.Copy(os.Stdout, reader); err != nil {
		return fmt.Errorf("PullImage ImagePull reader copy: %w", err)
	}

	return nil
}

// RunBuild runs the LaTeX build.
// It usese the files and directories specified in `options' on the
// host system as the source and destination for its job.
// It requires that options.WorkDir and options.TexDir are specified,
// otherwise it will return an error. It also requires that there is
// no directory "shared" in the project source root
//
// It returns the output of latexmk, or an error if the build fails.
func RunBuild(ctx context.Context, options BuildOptions) (string, error) {
	if options.TexDir == "" {
		return "", errors.New("BuildOptions.TexDir empty")
	}
	if options.OutDir == "" {
		return "", errors.New("BuildOptions.OutDir empty")
	}

	sharedDir := path.Join(options.TexDir, "shared")

	if _, err := os.Stat(sharedDir); err == nil {
		return "", errors.New("TeXDir/shared already exists")
	}

	if err := os.Mkdir(sharedDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("RunBuild failed to create shared directory: %w", err)
	}

	defer os.Remove(sharedDir)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("RunBuild NewClientWithOpts: %w", err)
	}
	defer cli.Close()

	var engine string
	if options.Engine == "" {
		engine = string(EnginePDF)
	} else {
		engine = string(options.Engine)
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

	mounts := []mount.Mount{
		{
			Type: mount.TypeBind,
			Source: options.TexDir,
			Target: "/workdir",
			ReadOnly: true,
		},
		{
			Type: mount.TypeBind,
			Target: "/mnt/out",
			Source: options.OutDir,
		},
		mountAux,
	}

	if options.SharedDir != "" {
		mounts = append(mounts, mount.Mount{
			Type: mount.TypeBind,
			Target: "/workdir/shared",
			Source: options.SharedDir,
		})
	}

	cmd := []string{"latexmk", engine, "-interaction=batchmode", "-file-line-error", "-auxdir=/mnt/aux", "-outdir=/mnt/out"};

	if options.Document != "" {
		cmd = append(cmd, options.Document)
	}

	if options.Force {
		cmd = append(cmd, "-f")
	}

	resp, err := cli.ContainerCreate(
		ctx,
		&container.Config{
			Image: TexLiveContainer,
			Cmd:   cmd,
			Tty:   false,
			NetworkDisabled: true,
		},
		&container.HostConfig{
			AutoRemove: true,
			Mounts: mounts,
		}, nil, nil, "",
	)
	if err != nil {
		return "", fmt.Errorf("RunBuild ContainerCreate: %w", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("RunBuild ContainerStart: %w", err)
	}

	// If the incoming context gets canceled, kill the container.
	// If the container doesn't get killed within 5 seconds, something
	// is wrong, kill the server to stop dead containers from building up
	finishedCh := make(chan struct{})
	go func() {
		select {
		case <- ctx.Done():
			log.Printf("RunBuild killing container %s: %s\n", resp.ID, ctx.Err())
			killCtx, canc := context.WithTimeout(context.Background(), 5 * time.Second)
			if err := cli.ContainerKill(killCtx, resp.ID, "KILL"); err != nil {
				log.Fatalf("RunBuild ContainerKill: %s", err)
			}
			canc()
		case <- finishedCh:
			// processing finished without issue
		}
	}()

	// Signal to the above goroutine that the job finished normally
	defer func(){
		if ctx.Err() == nil {
			finishedCh <- struct{}{}
		}
	}()

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return "", fmt.Errorf("RunBuild ContainerLogs: %w", err)
	}

	var outBuffer bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuffer, &outBuffer, out); err != nil {
		return "", fmt.Errorf("RunBuild ContainerLogs StdCopy: %w", err)
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", fmt.Errorf("RunBuild ContainerWait: %w", err)
		}
	case status := <-statusCh:
		if status.Error != nil {
			return "", fmt.Errorf("RunBuild ContainerWait status: %w", err)
		}
		if status.StatusCode != 0 {
			return "", fmt.Errorf("RunBuild ContainerWait non-zero exit: %d", status.StatusCode)
		}
	case <- ctx.Done():
		// Context is ended, the container killer goroutine should
		// kill the container for us
		return "", fmt.Errorf("RunBuild ContainerWait context ended: %w", ctx.Err())
	}

	return outBuffer.String(), nil
}

func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdir(1)

	if err == io.EOF {
		return true, nil
	}
	return false, err
}
