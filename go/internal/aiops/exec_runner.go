package aiops

import (
	"bytes"
	"context"
	"os/exec"
)

// ExecRunner is the default CommandRunner, backed by os/exec -- the real
// implementation SESSION-55 wires in for interactive use. Tests use a
// fake CommandRunner instead so package tests never depend on git/
// python3 being installed or on live subprocess behavior.
type ExecRunner struct {
	Dir string
}

func (r ExecRunner) Run(ctx context.Context, name string, args ...string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil
		}
	}
	return stdout.String(), stderr.String(), exitCode, err
}
