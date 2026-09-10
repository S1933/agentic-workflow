// Package runtime adapts installed coding CLIs. No provider SDKs are used.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type Request struct{ Model, Cwd, Prompt string }
type Command struct {
	Executable string
	Args       []string
	Stdin      string
}
type Adapter interface{ Command(Request) Command }
type Result struct {
	ExitCode    int
	Interrupted bool
}

func Lookup(name string) (Adapter, error) {
	switch name {
	case "claude-code":
		return ClaudeCode{}, nil
	case "codex":
		return Codex{}, nil
	case "opencode":
		return OpenCode{}, nil
	}
	return nil, fmt.Errorf("unsupported runtime %q", name)
}
func Available(name string) error {
	a, err := Lookup(name)
	if err != nil {
		return err
	}
	_, err = exec.LookPath(a.Command(Request{}).Executable)
	return err
}
func Run(ctx context.Context, a Adapter, r Request, stdout, stderr io.Writer) (Result, error) {
	spec := a.Command(r)
	cmd := exec.CommandContext(ctx, spec.Executable, spec.Args...)
	cmd.Dir = r.Cwd
	cmd.Stdin = strings.NewReader(spec.Stdin)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Kill the complete runtime process group on cancellation, including tool children.
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	result := Result{Interrupted: ctx.Err() != nil}
	if err != nil {
		result.ExitCode = -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			result.ExitCode = exit.ExitCode()
		}
		return result, fmt.Errorf("%s: %w", spec.Executable, err)
	}
	return result, nil
}
