package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fake struct {
	executable string
	args       []string
	stdin      string
}

func (f fake) Command(Request) Command { return Command{f.executable, f.args, f.stdin} }
func TestAdapters(t *testing.T) {
	for _, name := range []string{"claude-code", "codex", "opencode"} {
		t.Run(name, func(t *testing.T) {
			a, err := Lookup(name)
			if err != nil {
				t.Fatal(err)
			}
			prompt := "-x 'quoted' $(touch SHOULD_NOT_EXIST)\nsecond line"
			spec := a.Command(Request{Model: "chosen-model", Prompt: prompt})
			if !strings.Contains(strings.Join(spec.Args, " "), "chosen-model") {
				t.Fatal("lost model")
			}
			if spec.Stdin != prompt && spec.Args[len(spec.Args)-1] != prompt {
				t.Fatal("lost prompt")
			}
			for _, arg := range spec.Args {
				if arg == "--continue" || arg == "--session" || arg == "resume" || strings.Contains(arg, "bypass") || arg == "--auto" {
					t.Fatalf("unsafe session/permission flag: %s", arg)
				}
			}
			if strings.Contains(strings.Join(a.Command(Request{Model: "default"}).Args, " "), "--model") {
				t.Fatal("default sent as literal model")
			}
		})
	}
}
func TestSubprocessIOAndCwd(t *testing.T) {
	dir := t.TempDir()
	var out, diagnostics bytes.Buffer
	script := `printf '%s\n' "$PWD"; printf '%s\n' "$1"; cat; printf 'diagnostic' >&2; exit 7`
	prompt := "$(touch injected)\nlong context"
	result, err := Run(context.Background(), fake{"/bin/sh", []string{"-c", script, "test", prompt}, prompt}, Request{Cwd: dir}, &out, &diagnostics)
	if err == nil || result.ExitCode != 7 || !strings.Contains(out.String(), prompt) || !strings.Contains(out.String(), filepath.Base(dir)) || diagnostics.String() != "diagnostic" {
		t.Fatalf("%+v %v %q %q", result, err, out.String(), diagnostics.String())
	}
	if _, err = os.Stat(filepath.Join(dir, "injected")); !os.IsNotExist(err) {
		t.Fatal("executed prompt as shell")
	}
}
func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	start := time.Now()
	result, err := Run(ctx, fake{"/bin/sh", []string{"-c", "sleep 30 & wait"}, ""}, Request{Cwd: t.TempDir()}, &out, &out)
	if err == nil || !result.Interrupted || time.Since(start) > 3*time.Second {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestAdapterExecutables(t *testing.T) {
	for _, name := range []string{"claude-code", "codex", "opencode"} {
		t.Run(name, func(t *testing.T) {
			bin := t.TempDir()
			a, _ := Lookup(name)
			exe := a.Command(Request{}).Executable
			// The shell is only the test double, not how the production runner launches CLIs.
			script := "#!/bin/sh\nprintf 'cwd=%s\\n' \"$PWD\"\nprintf 'arg=%s\\n' \"$@\"\ncat\nprintf 'diagnostics' >&2\n"
			if err := os.WriteFile(filepath.Join(bin, exe), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			if err := Available(name); err != nil {
				t.Fatal(err)
			}
			project := t.TempDir()
			prompt := "-option-like $(touch INJECTED)\nsecond line"
			var out, diagnostics bytes.Buffer
			result, err := Run(context.Background(), a, Request{Model: "chosen", Cwd: project, Prompt: prompt}, &out, &diagnostics)
			if err != nil || result.ExitCode != 0 {
				t.Fatalf("%+v %v", result, err)
			}
			if !strings.Contains(out.String(), prompt) || !strings.Contains(out.String(), "arg=chosen") || !strings.Contains(out.String(), filepath.Base(project)) || diagnostics.String() != "diagnostics" {
				t.Fatalf("invalid transport: %q / %q", out.String(), diagnostics.String())
			}
			if _, err := os.Stat(filepath.Join(project, "INJECTED")); !os.IsNotExist(err) {
				t.Fatal("prompt executed as code")
			}
		})
	}
}
