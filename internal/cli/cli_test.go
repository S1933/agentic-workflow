package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/S1933/agentic-workflow/internal/config"
	"github.com/S1933/agentic-workflow/internal/orchestrator"
	agentruntime "github.com/S1933/agentic-workflow/internal/runtime"
	"github.com/S1933/agentic-workflow/internal/state"
)

type harness struct {
	t                        *testing.T
	project, base, configDir string
	calls                    []agentruntime.Request
	fail                     bool
	unavailable              bool
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	configDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return &harness{t: t, project: t.TempDir(), base: t.TempDir(), configDir: configDir}
}
func (h *harness) run(command, input string, interactive bool) (string, error) {
	h.t.Helper()
	var out, diagnostics bytes.Buffer
	app := App{In: strings.NewReader(input), Out: &out, Err: &diagnostics, Cwd: h.project, Interactive: interactive,
		Available: func(string) error {
			if h.unavailable {
				return fmt.Errorf("missing executable")
			}
			return nil
		},
		RunRuntime: func(_ context.Context, _ agentruntime.Adapter, r agentruntime.Request, out, errout io.Writer) (agentruntime.Result, error) {
			h.calls = append(h.calls, r)
			fmt.Fprintln(out, "Agent report: evidence and proposed work.")
			if h.fail {
				return agentruntime.Result{ExitCode: 7}, fmt.Errorf("fake runtime failed")
			}
			return agentruntime.Result{}, nil
		},
	}
	err := app.Run(context.Background(), []string{command, "--config-dir", h.configDir, "--state-dir", h.base})
	return out.String(), err
}
func (h *harness) must(command, input string) string {
	h.t.Helper()
	out, err := h.run(command, input, true)
	if err != nil {
		h.t.Fatalf("%s failed: %v\n%s", command, err, out)
	}
	return out
}
func (h *harness) load() (*state.Store, *state.State, *config.Config) {
	h.t.Helper()
	store, err := state.Open(h.base, h.project)
	if err != nil {
		h.t.Fatal(err)
	}
	s, err := store.Load()
	if err != nil {
		h.t.Fatal(err)
	}
	c, err := config.Load(store.RunDir(s))
	if err != nil {
		h.t.Fatal(err)
	}
	return store, s, c
}
func TestStartAcceptAndResume(t *testing.T) {
	h := newHarness(t)
	h.must("debug", "run\nreproducible bug\ninvestigate failure\ncodex\nyes\n")
	_, s, _ := h.load()
	if s.CurrentStep != "reproduction" || s.Status != "awaiting_acceptance" || len(h.calls) != 1 {
		t.Fatalf("unexpected state %+v", s)
	}
	h.must("resume", "accept\nyes\n")
	_, s, _ = h.load()
	if s.CurrentStep != "reproduction" || s.Step("reproduction").Status != "completed" || len(h.calls) != 1 {
		t.Fatal("accept launched next action")
	}
	h.must("resume", "next\nrun\nyes\nobserved stack trace\ncodex\nyes\n")
	_, s, _ = h.load()
	if s.CurrentStep != "analysis" || len(h.calls) != 2 {
		t.Fatal("resume did not run analysis")
	}
	if !strings.Contains(h.calls[1].Prompt, "Agent report") || h.calls[1].Cwd != s.Cwd {
		t.Fatal("missing context/cwd")
	}
	// Original configs can disappear: resume/status must use the snapshot.
	h.configDir = filepath.Join(h.base, "missing-config")
	if _, err := h.run("status", "", false); err != nil {
		t.Fatal(err)
	}
	h.must("resume", "pause\n")
	entries, err := os.ReadDir(h.project)
	if err != nil || len(entries) != 0 {
		t.Fatal("wrote Agentic files into target")
	}
}
func TestNoImplicitApproval(t *testing.T) {
	h := newHarness(t)
	if _, err := h.run("debug", "", false); err == nil {
		t.Fatal("accepted noninteractive start")
	}
	h.must("debug", "pause\n")
	if _, err := h.run("resume", "\n", true); err == nil {
		t.Fatal("empty input accepted")
	}
	if len(h.calls) != 0 {
		t.Fatal("launched without approval")
	}
	if _, err := h.run("feature", "pause\n", true); err == nil {
		t.Fatal("replaced active workflow")
	}
	h.must("reset", "no\n")
	h.load()
	store, s, _ := h.load()
	h.must("reset", "yes\n")
	if _, err := store.Load(); !os.IsNotExist(err) {
		t.Fatal("reset left active state")
	}
	if _, err := os.Stat(filepath.Join(store.RunDir(s), "archived-state.json")); err != nil {
		t.Fatal("reset lost archive")
	}
}
func TestUnavailableAndFailedRuntime(t *testing.T) {
	h := newHarness(t)
	h.unavailable = true
	if _, err := h.run("debug", "run\nreport\nrequest\ncodex\n", true); err == nil {
		t.Fatal("missing runtime accepted")
	}
	if len(h.calls) != 0 {
		t.Fatal("silently substituted runtime")
	}
	h.unavailable = false
	h.fail = true
	if _, err := h.run("resume", "run\ncodex\nyes\n", true); err == nil {
		t.Fatal("runtime failure swallowed")
	}
	_, s, _ := h.load()
	if s.Status != "failed" {
		t.Fatal("failure not saved")
	}
	h.must("resume", "pause\n")
	if len(h.calls) != 1 {
		t.Fatal("retried automatically")
	}
}

func TestExplicitFallback(t *testing.T) {
	h := newHarness(t)
	h.must("debug", "pause\n")
	store, s, _ := h.load()
	s.CurrentStep = "implementation"
	s.Step("plan").Status = "completed"
	for _, key := range []string{"request", "bug_report", "available_ticket_context", "root_cause", "correction_plan", "available_technical_context"} {
		s.Context[key] = "provided " + key
	}
	s.Step("implementation").Tasks = []state.Task{{ID: "fix", Text: "only this fix", Status: "pending"}}
	if err := store.Save(s); err != nil {
		t.Fatal(err)
	}
	h.fail = true
	if _, err := h.run("resume", "run\nyes\n", true); err == nil {
		t.Fatal("expected failed worker")
	}
	h.must("resume", "pause\n")
	if len(h.calls) != 1 {
		t.Fatal("automatic fallback")
	}
	h.fail = false
	h.must("resume", "fallback\nrepair the worker failure\ncodex\nyes\n")
	_, s, _ = h.load()
	attempts := s.Step("implementation").Attempts
	if len(attempts) != 2 || attempts[0].Runtime != "opencode" || attempts[1].Runtime != "codex" || attempts[1].Role != "expert" || attempts[0].TaskID != attempts[1].TaskID {
		t.Fatalf("bad escalation: %+v", attempts)
	}
	if !strings.Contains(h.calls[1].Prompt, "human explicitly authorized fallback") {
		t.Fatal("fallback authority absent from prompt")
	}
	h.must("resume", "accept\nyes\n")
}

func TestCancelWhileWaitingForHuman(t *testing.T) {
	h := newHarness(t)
	in, writer := io.Pipe()
	defer in.Close()
	defer writer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	app := App{In: in, Out: io.Discard, Err: io.Discard, Cwd: h.project, Interactive: true}
	err := app.Run(ctx, []string{"debug", "--config-dir", h.configDir, "--state-dir", h.base})
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected cancellation, got %v", err)
	}
	store, s, _ := h.load()
	if s.Status != "ready" {
		t.Fatal("lost state after prompt cancellation")
	}
	unlock, err := store.Lock()
	if err != nil {
		t.Fatal("cancellation retained project lock", err)
	}
	unlock()
}
func TestCompleteWorkflows(t *testing.T) {
	for _, workflow := range []string{"feature", "debug"} {
		t.Run(workflow, func(t *testing.T) {
			h := newHarness(t)
			// A clean Git project gives review an explicit, reproducible target.
			git := func(args ...string) {
				t.Helper()
				cmd := exec.Command("git", append([]string{"-C", h.project}, args...)...)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git: %v %s", err, out)
				}
			}
			git("init", "--quiet")
			git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "base", "--quiet")
			h.must(workflow, "pause\n")
			// Exercise CLI actions using the real YAML, without a model or issue tracker.
			for iteration := 0; iteration < 80; iteration++ {
				_, s, c := h.load()
				if s.Status == "completed" {
					break
				}
				e := orchestrator.Engine{Config: c, State: s}
				step, err := e.Current()
				if err != nil {
					t.Fatal(err)
				}
				ss := s.Step(step.Name)
				var script strings.Builder
				appendPrerequisites := func() {
					for _, key := range e.Missing(step) {
						switch c.Workflow.Conditions[key].Kind {
						case "input":
							fmt.Fprintf(&script, "provided %s\n", key)
						case "human":
							script.WriteString("yes\nverified with evidence\n")
						default:
							t.Fatalf("unreachable step %s: missing %s", step.Name, key)
						}
					}
				}
				switch ss.Status {
				case "completed", "skipped":
					script.WriteString("next\npause\n")
				case "awaiting_acceptance":
					script.WriteString("accept\n")
					appendPrerequisites()
					script.WriteString("yes\n")
				case "pending":
					// Include two backend and implementation tasks; skip frontend and corrections.
					if step.Name == "interview" || step.Name == "frontend" || step.Name == "corrections" || step.Name == "correction_review" {
						script.WriteString("skip\n")
						if step.When != "" && c.Workflow.Conditions[step.When].Kind == "human" {
							script.WriteString("no\nno blocking or major findings\n")
						}
					} else if step.Role == "" || step.Mode != "" {
						script.WriteString("validate\n")
						appendPrerequisites()
						script.WriteString("yes\n")
					} else {
						script.WriteString("run\n")
						appendPrerequisites()
						if s.Context["request"] == "" {
							script.WriteString("requested objective\n")
						}
						keys := append([]string{}, step.Context...)
						if step.SourceOfTruth != "" {
							keys = append(keys, step.SourceOfTruth)
						}
						for _, key := range keys {
							if key != "child_task" && s.Context[key] == "" {
								fmt.Fprintf(&script, "context for %s\n", key)
							}
						}
						if step.TaskMode != "" && len(ss.Tasks) == 0 {
							script.WriteString("2\nfirst isolated task\nsecond isolated task\nyes\n")
						}
						if step.Input["diff"] != "" {
							script.WriteString("HEAD\n")
						}
						candidates, _ := c.Candidates(step.Role)
						if len(candidates) > 1 {
							script.WriteString("codex\n")
						}
						script.WriteString("yes\n")
					}
				default:
					t.Fatalf("unexpected state %s", ss.Status)
				}
				before := len(h.calls)
				h.must("resume", script.String())
				if len(h.calls) > before+1 {
					t.Fatal("more than one task per invocation")
				}
			}
			_, s, _ := h.load()
			if s.Status != "completed" {
				t.Fatalf("workflow not completed: %s %s", s.CurrentStep, s.Status)
			}
			if s.Step("corrections").Status != "skipped" {
				t.Fatal("correction applicability lost")
			}
			var taskPrompts []string
			for _, call := range h.calls {
				if strings.Contains(call.Prompt, "Current task ") {
					taskPrompts = append(taskPrompts, call.Prompt)
				}
			}
			if len(taskPrompts) != 2 {
				t.Fatalf("expected two independent task invocations, got %d", len(taskPrompts))
			}
			if strings.Contains(taskPrompts[0], "second isolated task") || strings.Contains(taskPrompts[1], "first isolated task") {
				t.Fatal("task contexts leaked")
			}
		})
	}
}
