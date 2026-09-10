// Package orchestrator defines state transitions without launching agents or reading stdin.
package orchestrator

import (
	"fmt"
	"time"

	"github.com/S1933/agentic-workflow/internal/config"
	"github.com/S1933/agentic-workflow/internal/state"
)

type Engine struct {
	Config *config.Config
	State  *state.State
}

func (e Engine) Steps() []config.Step { return e.Config.Workflow.Workflows[e.State.Workflow].Ordered() }
func (e Engine) Current() (config.Step, error) {
	for _, s := range e.Steps() {
		if s.Name == e.State.CurrentStep {
			return s, nil
		}
	}
	return config.Step{}, fmt.Errorf("unknown active step %s", e.State.CurrentStep)
}
func (e Engine) Condition(key string) bool {
	c, ok := e.Config.Workflow.Conditions[key]
	if !ok {
		return false
	}
	done := func(name string) bool {
		s := e.State.Step(name)
		return s.Status == "completed" || (c.AllowSkipped && s.Status == "skipped")
	}
	switch c.Kind {
	case "input":
		return e.State.Context[key] != ""
	case "completion":
		return done(c.Step)
	case "aggregate":
		for _, s := range c.Steps {
			if !done(s) {
				return false
			}
		}
		return len(c.Steps) > 0
	case "human":
		if c.After != "" && e.State.Step(c.After).Status != "completed" {
			return false
		}
		return e.State.Conditions[key].Value
	}
	return false
}
func (e Engine) Attest(key, evidence string, value bool) error {
	c, ok := e.Config.Workflow.Conditions[key]
	if !ok || c.Kind != "human" {
		return fmt.Errorf("%s is not a human attestation", key)
	}
	if c.After != "" && e.State.Step(c.After).Status != "completed" {
		return fmt.Errorf("%s must be accepted first", c.After)
	}
	if evidence == "" {
		return fmt.Errorf("an attestation needs evidence or an explanation")
	}
	e.State.Conditions[key] = state.Attestation{Value: value, Evidence: evidence, At: time.Now().UTC()}
	e.State.Record("attest", fmt.Sprintf("%s=%t: %s", key, value, evidence))
	return nil
}
func (e Engine) Missing(s config.Step) []string {
	keys := append([]string{}, s.Requires...)
	// Interview may gather missing product requirements before analysis starts.
	if s.Name != "interview" {
		keys = append(keys, e.Config.Workflow.Workflows[e.State.Workflow].Requires...)
	}
	out := []string{}
	seen := map[string]bool{}
	for _, key := range keys {
		if !seen[key] && !e.Condition(key) {
			out = append(out, key)
			seen[key] = true
		}
	}
	return out
}
func (e Engine) Skip() error {
	s, err := e.Current()
	if err != nil {
		return err
	}
	if !s.Optional {
		return fmt.Errorf("%s is not optional", s.Name)
	}
	if e.State.Step(s.Name).Status != "pending" {
		return fmt.Errorf("cannot skip an attempted step; rewind it first")
	}
	for _, task := range e.State.Step(s.Name).Tasks {
		if task.Status != "pending" {
			return fmt.Errorf("cannot skip partially completed tasks; rewind first")
		}
	}
	if s.When != "" {
		c := e.Config.Workflow.Conditions[s.When]
		if c.Kind == "human" {
			if _, ok := e.State.Conditions[s.When]; !ok {
				return fmt.Errorf("condition %s needs a human decision", s.When)
			}
		}
		if e.Condition(s.When) {
			return fmt.Errorf("%s is required because %s is true", s.Name, s.When)
		}
	}
	e.State.Step(s.Name).Status = "skipped"
	e.State.Status = "awaiting_transition"
	e.State.Record("skip", s.Name)
	return nil
}
func (e Engine) Accept() error {
	s, err := e.Current()
	if err != nil {
		return err
	}
	if len(e.Missing(s)) > 0 {
		return fmt.Errorf("unmet prerequisites: %v", e.Missing(s))
	}
	ss := e.State.Step(s.Name)
	if s.Role != "" && s.Mode == "" && ss.Status != "awaiting_acceptance" {
		return fmt.Errorf("no successful result to accept")
	}
	if len(ss.Attempts) > 0 && ss.Status == "awaiting_acceptance" {
		ss.Attempts[len(ss.Attempts)-1].Status = "accepted"
	}
	if s.TaskMode != "" {
		task := ActiveTask(ss)
		if task == nil || task.Status != "awaiting_acceptance" {
			return fmt.Errorf("no task result to accept")
		}
		task.Status = "completed"
		ss.Feedback = ""
		e.State.Record("accept_task", task.ID)
		if ActiveTask(ss) != nil {
			ss.Status = "pending"
			e.State.Status = "awaiting_transition"
			return nil
		}
	}
	ss.Status = "completed"
	e.State.Status = "awaiting_transition"
	e.State.Record("accept_step", s.Name)
	steps := e.Steps()
	if s.Name == steps[len(steps)-1].Name {
		e.State.Status = "completed"
	}
	return nil
}
func (e Engine) Advance() error {
	s, err := e.Current()
	if err != nil {
		return err
	}
	status := e.State.Step(s.Name).Status
	if status != "completed" && status != "skipped" {
		return fmt.Errorf("current step is not accepted or skipped")
	}
	steps := e.Steps()
	for i, step := range steps {
		if step.Name == s.Name {
			if i == len(steps)-1 {
				return fmt.Errorf("workflow completed")
			}
			e.State.CurrentStep = steps[i+1].Name
			e.State.Status = "ready"
			e.State.Record("transition", s.Name+" -> "+e.State.CurrentStep)
			return nil
		}
	}
	return fmt.Errorf("step not found")
}
func ActiveTask(s *state.Step) *state.Task {
	for i := range s.Tasks {
		if s.Tasks[i].Status != "completed" {
			return &s.Tasks[i]
		}
	}
	return nil
}
func (e Engine) Begin(runtime, model string, fallback bool) (*state.Attempt, error) {
	s, err := e.Current()
	if err != nil {
		return nil, err
	}
	if s.Role == "" || s.Mode != "" {
		return nil, fmt.Errorf("human checkpoint cannot launch runtime")
	}
	if missing := e.Missing(s); len(missing) > 0 {
		return nil, fmt.Errorf("unmet prerequisites: %v", missing)
	}
	if s.When != "" && !e.Condition(s.When) {
		return nil, fmt.Errorf("step does not apply")
	}
	ss := e.State.Step(s.Name)
	if ss.Status != "pending" && ss.Status != "failed" && ss.Status != "interrupted" && ss.Status != "awaiting_acceptance" {
		return nil, fmt.Errorf("cannot execute step in state %s", ss.Status)
	}
	role := s.Role
	if fallback {
		if s.Fallback == nil || len(ss.Attempts) == 0 {
			return nil, fmt.Errorf("no fallback available")
		}
		role = s.Fallback.Role
		e.State.Record("human_decision", "fallback to "+role)
	}
	taskID := ""
	if s.TaskMode != "" {
		task := ActiveTask(ss)
		if task == nil {
			return nil, fmt.Errorf("no pending task")
		}
		taskID = task.ID
		task.Status = "running"
	}
	// Renew mutable observations, retaining historical facts such as publication
	// and findings in an already accepted review. Rewind invalidates all assertions.
	for key := range e.State.Conditions {
		if e.Config.Workflow.Conditions[key].InvalidateOnExecution {
			delete(e.State.Conditions, key)
		}
	}
	ss.Status = "running"
	e.State.Status = "running"
	a := state.Attempt{ID: fmt.Sprintf("%s-%04d", s.Name, len(ss.Attempts)+1), TaskID: taskID, Runtime: runtime, Model: model, Role: role, Status: "running", Started: time.Now().UTC()}
	ss.Attempts = append(ss.Attempts, a)
	e.State.Record("execute", a.ID)
	return &ss.Attempts[len(ss.Attempts)-1], nil
}
func (e Engine) Finish(exit int, interrupted bool) {
	ss := e.State.Step(e.State.CurrentStep)
	if len(ss.Attempts) == 0 {
		return
	}
	status := "awaiting_acceptance"
	if exit != 0 {
		status = "failed"
	}
	if interrupted {
		status = "interrupted"
	}
	a := &ss.Attempts[len(ss.Attempts)-1]
	a.ExitCode = exit
	a.Status = status
	ss.Status = status
	if task := ActiveTask(ss); task != nil {
		task.Status = status
	}
	e.State.Status = status
}

// Rewind clears downstream progress and assertions, preserving immutable attempt reports.
func (e Engine) Rewind(target string) error {
	current := -1
	dest := -1
	steps := e.Steps()
	for i, s := range steps {
		if s.Name == e.State.CurrentStep {
			current = i
		}
		if s.Name == target {
			dest = i
		}
	}
	if dest < 0 || current < dest {
		return fmt.Errorf("rewind target must be the current or an earlier step")
	}
	for _, s := range steps[dest:] {
		ss := e.State.Step(s.Name)
		ss.Status = "pending"
		ss.Tasks = nil
		ss.Feedback = ""
	}
	// Derived context (spec, plan, root cause, review diff) must be supplied again.
	for key := range e.State.Context {
		if e.Config.Workflow.Conditions[key].Kind != "input" && key != "request" {
			delete(e.State.Context, key)
		}
	}
	e.State.Conditions = map[string]state.Attestation{}
	e.State.CurrentStep = target
	e.State.Status = "ready"
	e.State.Record("rewind", target)
	return nil
}
