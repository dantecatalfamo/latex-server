package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/go-chi/chi/v5/middleware"
)

func RunBuildNative(ctx context.Context, options BuildOptions) (string, error) {
	var engineArg string
	switch options.Engine {
	case EnginePDF:
		engineArg = "-pdf"
	case EngineLua:
		if options.AllowLuaTex {
			engineArg = "-pdflua"
		} else {
			engineArg = "-pdf"
		}
	case EngineXeTeX:
		engineArg = "-pdfxe"
	default:
		engineArg = "-pdf"
	}

	auxDir := fmt.Sprintf("-auxdir=%s", options.AuxDir)
	outDir := fmt.Sprintf("-outdir=%s", options.OutDir)

	args := []string{engineArg, auxDir, outDir};

	if !options.AllowLatexmkrc {
		args = append(args, "-norc")
	}

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

	if options.Dependents {
		args = append(args, "-deps")
	}

	err := os.Chdir(options.SrcDir)
	if err != nil {
		return "", fmt.Errorf("RunBuild: %w", err)
	}

	cmd := exec.CommandContext(ctx, "latexmk", args...)

	cmdOut := new(bytes.Buffer)
	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut

	// HTTP request ID
	requestId := middleware.GetReqID(ctx)

	log.Printf("[%s] Starting build in %s: %v", requestId, options.SrcDir, args)
	if err := cmd.Run(); err != nil {
		// If error is type *ExitError, the cmdOut should be populated
		// with an error message
		return cmdOut.String(), fmt.Errorf("RunBuild: %w", err)
	}

	return cmdOut.String(), nil
}
