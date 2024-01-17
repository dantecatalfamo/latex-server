package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
)

type Engine string


// TODO Make these match the options sent in by clients and translated
// internally
const (
	EnginePDF Engine = "-pdf"
	EngineLua Engine = "-pdflua"
	EngineXeTeX Engine = "-pdfxe"
)

type BuildOptions struct {
	AuxDir string
	OutDir string
	SrcDir string
	SharedDir string
	Document string
	Engine Engine
	Force bool
	FileLineError bool
}

func RunBuild(ctx context.Context, options BuildOptions) (string, error) {
	engine := EnginePDF
	if (options.Engine != "") {
		engine = options.Engine
	}
	auxDir := fmt.Sprintf("-auxdir=%s", options.AuxDir)
	outDir := fmt.Sprintf("-outdir=%s", options.OutDir)

	args := []string{string(engine), auxDir, outDir, "-deps", "-norc"};

	if options.Document != "" {
		args = append(args, options.Document)
	}

	if options.Force {
		args = append(args, "-f", "-interaction=nonstopmode")
	} else {
		args = append(args, "-interaction=batchmode")
	}

	if options.FileLineError {
		args = append(args, "-file-line-error")
	}

	err := os.Chdir(options.SrcDir)
	if err != nil {
		return "", fmt.Errorf("RunBuild: %w", err)
	}

	cmd := exec.CommandContext(ctx, "latexmk", args...)

	cmdOut := new(bytes.Buffer)
	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut

	log.Printf("Starting build in %s: %v", options.SrcDir, args)
	if err := cmd.Run(); err != nil {
		// If error is type *ExitError, the cmdOut should be populated
		// with an error message
		return cmdOut.String(), fmt.Errorf("RunBuild: %w", err)
	}

	return cmdOut.String(), nil
}
