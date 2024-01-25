package server

import (
	"context"
	"errors"
	"fmt"
)

type Engine string


const (
	EnginePDF Engine = "pdf"
	EngineLua Engine = "lua"
	EngineXeTeX Engine = "xe"
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
	Dependents bool
	BuildMode BuildMode
	AllowLatexmkrc bool
	AllowLuaTex bool
}

func RunBuild(ctx context.Context, options BuildOptions) (string, error) {
	if options.BuildMode == BuildModeNative {
		return RunBuildNative(ctx, options)
	} else if options.BuildMode == BuildModeDocker {
		// TODO re-add docker build mode
		return "", errors.New("docker build mode not yet implemented")
	}
	return "", fmt.Errorf("invalid build mode \"%s\"", options.BuildMode)
}
