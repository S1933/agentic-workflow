// Package cli provides a small human-driven terminal interface.
package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/S1933/agentic-workflow/internal/config"
	promptcontext "github.com/S1933/agentic-workflow/internal/context"
	"github.com/S1933/agentic-workflow/internal/orchestrator"
	agentruntime "github.com/S1933/agentic-workflow/internal/runtime"
	"github.com/S1933/agentic-workflow/internal/state"
)

type Runner func(context.Context, agentruntime.Adapter, agentruntime.Request, io.Writer, io.Writer) (agentruntime.Result, error)
type App struct {
	In          io.Reader
	Out, Err    io.Writer
	Cwd         string
	Interactive bool
	RunRuntime  Runner
	Available   func(string) error
	reader      *bufio.Reader
	ctx         context.Context
}

func (a *App) Run(ctx context.Context, args []string) (returnErr error) {
	a.ctx = ctx
	if a.In == nil {
		a.In = os.Stdin
	}
	if a.Out == nil {
		a.Out = os.Stdout
	}
	if a.Err == nil {
		a.Err = os.Stderr
	}
	if a.RunRuntime == nil {
		a.RunRuntime = agentruntime.Run
	}
	if a.Available == nil {
		a.Available = agentruntime.Available
	}
	a.reader = bufio.NewReader(a.In)
	if a.Cwd == "" {
		var err error
		a.Cwd, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	flags := flag.NewFlagSet("agentic", flag.ContinueOnError)
	flags.SetOutput(a.Out)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := flags.String("config-dir", filepath.Join(home, ".agentic-workflow"), "directory containing workflow.yaml and runtime.yml")
	stateDir := flags.String("state-dir", "", "external state root (default: XDG_STATE_HOME/agentic-workflow)")
	flags.Usage = func() {
		fmt.Fprintln(a.Out, "Usage: agentic <feature|debug|status|resume|reset> [--config-dir DIR] [--state-dir DIR]\n\nOne agent task per invocation. Human decisions are always explicit.")
		flags.PrintDefaults()
	}
	// Accept both `agentic feature --flag value` and `agentic --flag value feature`.
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string{}, args[1:]...), args[0])
	}
	if err = flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return fmt.Errorf("one command is required")
	}
	command := flags.Arg(0)
	if command != "feature" && command != "debug" && command != "status" && command != "resume" && command != "reset" {
		return fmt.Errorf("unknown command %q", command)
	}
	store, err := state.Open(*stateDir, a.Cwd)
	if err != nil {
		return err
	}
	if command == "status" {
		s, err := store.Load()
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(a.Out, "No active workflow for this project.")
			return nil
		}
		if err != nil {
			return err
		}
		c, err := config.Load(store.RunDir(s))
		if err != nil {
			return err
		}
		return a.status(store, orchestrator.Engine{Config: c, State: s})
	}
	if !a.Interactive {
		return fmt.Errorf("%s requires an interactive terminal for human decisions; use status to inspect state", command)
	}
	unlock, err := store.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	var s *state.State
	var c *config.Config
	if command == "feature" || command == "debug" {
		c, err = config.Load(*configDir)
		if err != nil {
			return err
		}
		// Fail before creating an active run if one already exists.
		if _, err = store.Load(); err == nil {
			return fmt.Errorf("a workflow already exists; use resume or reset")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		s, err = store.Create(c, command, a.Cwd)
		if err != nil {
			return err
		}
	} else {
		s, err = store.Load()
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no active workflow; start feature or debug")
		}
		if err != nil {
			return err
		}
		if command == "reset" {
			yes, err := a.confirm("Archive this workflow and clear its active state? Project files will not be changed.")
			if err != nil {
				return err
			}
			if !yes {
				return nil
			}
			s.Record("reset", "archive active workflow")
			if err = store.Save(s); err != nil {
				return err
			}
			if err = store.Archive(s); err != nil {
				return err
			}
			fmt.Fprintf(a.Out, "Archived at %s\n", store.RunDir(s))
			return nil
		}
		c, err = config.Load(store.RunDir(s))
		if err != nil {
			return err
		}
	}
	defer func() {
		if saveErr := store.Save(s); saveErr != nil {
			returnErr = errors.Join(returnErr, saveErr)
		}
	}()
	e := orchestrator.Engine{Config: c, State: s}
	if s.Status == "running" {
		e.Finish(-1, true)
		s.Record("recover", "previous process ended without saving its result; inspect project before retrying")
	}
	if err = a.status(store, e); err != nil {
		return err
	}
	if s.Status == "completed" {
		return nil
	}
	return a.progress(ctx, store, e)
}
func (a *App) status(store *state.Store, e orchestrator.Engine) error {
	s := e.State
	step, err := e.Current()
	if err != nil {
		return err
	}
	ss := s.Step(step.Name)
	fmt.Fprintf(a.Out, "Workflow: %s\nProject: %s\nRuntime cwd: %s\nStep: %s (%s)\nState: %s\nArtifacts: %s\n", s.Workflow, s.ProjectRoot, s.Cwd, step.Name, ss.Status, s.Status, store.RunDir(s))
	if task := orchestrator.ActiveTask(ss); task != nil {
		fmt.Fprintf(a.Out, "Task: %s (%s) — %s\n", task.ID, task.Status, task.Text)
	}
	if len(ss.Attempts) > 0 {
		last := ss.Attempts[len(ss.Attempts)-1]
		fmt.Fprintf(a.Out, "Last result: %s / %s, exit %d\n%s\n", last.Runtime, last.Model, last.ExitCode, filepath.Join(store.RunDir(s), "attempts", last.ID, "stdout.txt"))
	}
	if missing := e.Missing(step); len(missing) > 0 {
		fmt.Fprintf(a.Out, "Missing prerequisites: %s\n", strings.Join(missing, ", "))
	}
	fmt.Fprintln(a.Out, "Human action: use resume to accept a result, authorize the next action, retry or rewind.")
	return nil
}
func (a *App) line(question string) (string, error) {
	fmt.Fprintf(a.Out, "%s\n> ", question)
	type answer struct {
		text string
		err  error
	}
	read := make(chan answer, 1)
	go func() { text, err := a.reader.ReadString('\n'); read <- answer{text, err} }()
	select {
	case <-a.ctx.Done():
		return "", a.ctx.Err()
	case result := <-read:
		if result.err != nil {
			return "", fmt.Errorf("human input required: %w", result.err)
		}
		return strings.TrimSpace(result.text), nil
	}
}
func (a *App) choose(question string, options ...string) (string, error) {
	for {
		answer, err := a.line(question + " [" + strings.Join(options, " / ") + "]")
		if err != nil {
			return "", err
		}
		for _, o := range options {
			if answer == o {
				return answer, nil
			}
		}
		fmt.Fprintln(a.Out, "Choose an explicit listed action; empty input does not approve anything.")
	}
}
func (a *App) confirm(question string) (bool, error) {
	answer, err := a.choose(question, "yes", "no")
	return answer == "yes", err
}
func (a *App) text(question string, required bool) (string, error) {
	for {
		value, err := a.line(question + " (text or @path to a UTF-8 file)")
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(value, "@") {
			path := strings.TrimPrefix(value, "@")
			if !filepath.IsAbs(path) {
				path = filepath.Join(a.Cwd, path)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			value = strings.TrimSpace(string(data))
		}
		if value != "" || !required {
			return value, nil
		}
		fmt.Fprintln(a.Out, "A non-empty value is required.")
	}
}
func (a *App) prerequisites(e orchestrator.Engine, step config.Step) error {
	for _, key := range e.Missing(step) {
		c := e.Config.Workflow.Conditions[key]
		switch c.Kind {
		case "input":
			value, err := a.text("Provide "+key, true)
			if err != nil {
				return err
			}
			e.State.Context[key] = value
			e.State.Record("input", key)
		case "human":
			if err := a.attest(e, key); err != nil {
				return err
			}
			if !e.Condition(key) {
				return fmt.Errorf("blocked by %s; resume can rewind to the relevant step", key)
			}
		default:
			return fmt.Errorf("blocked by %s; accept the required step or rewind first", key)
		}
	}
	return nil
}
func (a *App) attest(e orchestrator.Engine, key string) error {
	c := e.Config.Workflow.Conditions[key]
	if c.After != "" && e.State.Step(c.After).Status != "completed" {
		return fmt.Errorf("%s requires accepted step %s", key, c.After)
	}
	yes, err := a.confirm("Confirm condition: " + key + "?")
	if err != nil {
		return err
	}
	evidence, err := a.text("Evidence or explanation for "+key, true)
	if err != nil {
		return err
	}
	return e.Attest(key, evidence, yes)
}
func (a *App) progress(ctx context.Context, store *state.Store, e orchestrator.Engine) error {
	step, err := e.Current()
	if err != nil {
		return err
	}
	ss := e.State.Step(step.Name)
	options := []string{"pause", "rewind", "edit"}
	switch ss.Status {
	case "completed", "skipped":
		options = append(options, "next")
	case "awaiting_acceptance":
		options = append(options, "accept", "retry")
		if step.Fallback != nil {
			options = append(options, "fallback")
		}
	case "failed", "interrupted":
		options = append(options, "retry")
		if step.Fallback != nil {
			options = append(options, "fallback")
		}
	case "pending":
		if step.Role == "" || step.Mode != "" {
			options = append(options, "validate")
		} else {
			options = append(options, "run")
		}
		if step.Optional {
			options = append(options, "skip")
		}
	default:
		return fmt.Errorf("unsupported step state %s", ss.Status)
	}
	action, err := a.choose("Choose the action for "+step.Name, options...)
	if err != nil {
		return err
	}
	switch action {
	case "pause":
		return nil
	case "rewind":
		target, err := a.line("Return to which current/earlier step? This invalidates its results and subsequent validations.")
		if err != nil {
			return err
		}
		yes, err := a.confirm("Confirm rewind to " + target + "?")
		if err != nil || !yes {
			return err
		}
		return e.Rewind(target)
	case "edit":
		// Product inputs affect the entire workflow, so no later approval survives.
		target := e.Steps()[0].Name
		key, err := a.line("Input key to replace (request, functional_request, acceptance_criteria or bug_report)?")
		if err != nil {
			return err
		}
		if key != "request" && e.Config.Workflow.Conditions[key].Kind != "input" {
			return fmt.Errorf("unsupported input key %s", key)
		}
		value, err := a.text("Replacement for "+key, true)
		if err != nil {
			return err
		}
		yes, err := a.confirm("Replace input and invalidate progress from " + target + "?")
		if err != nil || !yes {
			return err
		}
		if err = e.Rewind(target); err != nil {
			return err
		}
		e.State.Context[key] = value
		e.State.Record("edit_input", key)
		return nil
	case "next":
		if err = e.Advance(); err != nil {
			return err
		}
		// Moving the pointer is a separate explicit decision from running the next task.
		return a.progress(ctx, store, e)
	case "skip":
		if step.When != "" && e.Config.Workflow.Conditions[step.When].Kind == "human" {
			if err = a.attest(e, step.When); err != nil {
				return err
			}
		}
		return e.Skip()
	case "accept", "validate":
		if err = a.prerequisites(e, step); err != nil {
			return err
		}
		yes, err := a.confirm("Explicitly accept " + step.Name + " and its current task/result?")
		if err != nil || !yes {
			return err
		}
		return e.Accept()
	case "run", "retry", "fallback":
		if step.When != "" {
			if e.Config.Workflow.Conditions[step.When].Kind == "human" && !e.Condition(step.When) {
				if err = a.attest(e, step.When); err != nil {
					return err
				}
			}
			if !e.Condition(step.When) {
				return fmt.Errorf("%s does not apply; choose skip", step.Name)
			}
		}
		if err = a.prerequisites(e, step); err != nil {
			return err
		}
		if action != "run" {
			feedback, err := a.text("Instructions for this new attempt (include interview answers if needed)", true)
			if err != nil {
				return err
			}
			ss.Feedback = feedback
		}
		if err = a.collectContext(e, step); err != nil {
			return err
		}
		return a.execute(ctx, store, e, step, action == "fallback")
	}
	return nil
}
func (a *App) collectContext(e orchestrator.Engine, step config.Step) error {
	s := e.State
	if s.Context["request"] == "" {
		value, err := a.text("Describe the objective or attach the ticket/report", true)
		if err != nil {
			return err
		}
		s.Context["request"] = value
	}
	// parent_ticket/spec are explicit external source material, never a claimed publication.
	keys := append([]string{}, step.Context...)
	if step.SourceOfTruth != "" {
		keys = append(keys, step.SourceOfTruth)
	}
	for _, key := range keys {
		if key == "child_task" || s.Context[key] != "" {
			continue
		}
		required := key == "parent_ticket" || key == "spec" || key == "acceptance_criteria" || key == "bug_report" || key == "root_cause" || key == "correction_plan"
		value, err := a.text("Provide context: "+key, required)
		if err != nil {
			return err
		}
		if value != "" {
			s.Context[key] = value
		}
	}
	ss := s.Step(step.Name)
	if step.TaskMode != "" && len(ss.Tasks) == 0 {
		if step.Name == "corrections" {
			fmt.Fprintln(a.Out, "Enter blocking findings first, then major findings, as separate correction tasks. Minor findings remain in the report.")
		}
		value, err := a.line("Number of individually approved tasks for " + step.Name + "?")
		if err != nil {
			return err
		}
		count, err := strconv.Atoi(value)
		if err != nil || count < 1 || count > 1000 {
			return fmt.Errorf("task count must be between 1 and 1000")
		}
		tasks := make([]state.Task, 0, count)
		for i := 1; i <= count; i++ {
			text, err := a.text(fmt.Sprintf("Task %d: objective and acceptance criteria", i), true)
			if err != nil {
				return err
			}
			tasks = append(tasks, state.Task{ID: fmt.Sprintf("%s-%d", step.Name, i), Text: text, Status: "pending"})
		}
		yes, err := a.confirm("Approve this ordered task list?")
		if err != nil {
			return err
		}
		if !yes {
			return fmt.Errorf("task list was not approved")
		}
		ss.Tasks = tasks
		s.Record("approve_tasks", step.Name)
	}
	if step.Input["diff"] == "full_against_target_branch" {
		return a.reviewContext(s)
	}
	return nil
}
func (a *App) reviewContext(s *state.State) error {
	target, err := a.line("Target branch/ref for the full review diff?")
	if err != nil {
		return err
	}
	if target == "" || strings.HasPrefix(target, "-") {
		return fmt.Errorf("invalid target ref")
	}
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", s.ProjectRoot}, args...)...)
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("git %v: %w", args, err)
		}
		return string(out), nil
	}
	sha, err := git("rev-parse", "--verify", "--end-of-options", target+"^{commit}")
	if err != nil {
		return err
	}
	sha = strings.TrimSpace(sha)
	status, err := git("status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("review requires a clean working tree so its full branch diff is unambiguous; commit or otherwise resolve changes yourself, then resume")
	}
	base, err := git("merge-base", sha, "HEAD")
	if err != nil {
		return err
	}
	diff, err := git("diff", "--no-ext-diff", "--no-textconv", "--binary", strings.TrimSpace(base), "HEAD", "--")
	if err != nil {
		return err
	}
	s.Context["review_target"] = target + " (" + sha + ")"
	s.Context["review_diff"] = diff
	s.Record("review_scope", s.Context["review_target"])
	return nil
}
func (a *App) execute(ctx context.Context, store *state.Store, e orchestrator.Engine, step config.Step, fallback bool) error {
	role := step.Role
	if fallback {
		role = step.Fallback.Role
	}
	candidates, err := e.Config.Candidates(role)
	if err != nil {
		return err
	}
	chosen := candidates[0]
	if len(candidates) > 1 {
		names := []string{}
		for _, candidate := range candidates {
			fmt.Fprintf(a.Out, "%s: %s\n", candidate.Name, candidate.Model)
			names = append(names, candidate.Name)
		}
		name, err := a.choose("Select runtime for "+role, names...)
		if err != nil {
			return err
		}
		for _, candidate := range candidates {
			if candidate.Name == name {
				chosen = candidate
			}
		}
	}
	if err = a.Available(chosen.Name); err != nil {
		return fmt.Errorf("selected runtime %s unavailable: %w", chosen.Name, err)
	}
	adapter, err := agentruntime.Lookup(chosen.Name)
	if err != nil {
		return err
	}
	prompt, err := promptcontext.Build(e.Config, e.State, step, role, store.RunDir(e.State))
	if err != nil {
		return err
	}
	if fallback {
		prompt = "The human explicitly authorized fallback implementation by an expert for this task.\n" + prompt
	}
	yes, err := a.confirm(fmt.Sprintf("Launch one fresh %s action using %s (%s) in %s?", role, chosen.Name, chosen.Model, e.State.Cwd))
	if err != nil || !yes {
		return err
	}
	attempt, err := e.Begin(chosen.Name, chosen.Model, fallback)
	if err != nil {
		return err
	}
	// From here, failures must never leave a seemingly successful attempt behind.
	result := agentruntime.Result{ExitCode: -1}
	defer func() { e.Finish(result.ExitCode, result.Interrupted) }()
	dir := filepath.Join(store.RunDir(e.State), "attempts", attempt.ID)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err = state.Atomic(filepath.Join(dir, "prompt.txt"), []byte(prompt)); err != nil {
		return err
	}
	stdout, err := os.OpenFile(filepath.Join(dir, "stdout.txt"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(filepath.Join(dir, "stderr.txt"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer stderr.Close()
	if err = store.Save(e.State); err != nil {
		return err
	}
	result, err = a.RunRuntime(ctx, adapter, agentruntime.Request{Model: chosen.Model, Cwd: e.State.Cwd, Prompt: prompt}, io.MultiWriter(stdout, a.Out), io.MultiWriter(stderr, a.Err))
	if syncErr := errors.Join(stdout.Sync(), stderr.Sync()); syncErr != nil {
		result.ExitCode = -1
		return errors.Join(err, syncErr)
	}
	fmt.Fprintf(a.Out, "\nResult saved at %s. No transition has been authorized. Use agentic resume.\n", dir)
	return err
}
