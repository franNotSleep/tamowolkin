package agents

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func runGit(dir string, args ...string) ([]byte, error) {
	return runCommand(dir, "git", args...)
}

func runGH(dir string, args ...string) ([]byte, error) {
	return runCommand(dir, "gh", args...)
}

func runCommand(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return out, fmt.Errorf("%s %s exited %d: %s", name, strings.Join(args, " "), exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return out, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out, nil
}
