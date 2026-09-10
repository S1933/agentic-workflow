package orchestrator

import (
	"testing"

	"github.com/S1933/agentic-workflow/internal/config"
	"github.com/S1933/agentic-workflow/internal/state"
)

func engine(t *testing.T, workflow, step string) Engine {
	t.Helper()
	c, err := config.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	return Engine{c, &state.State{Workflow: workflow, CurrentStep: step, Steps: map[string]*state.Step{}, Context: map[string]string{}, Conditions: map[string]state.Attestation{}}}
}
func TestHumanGates(t *testing.T) {
	e := engine(t, "feature", "analysis")
	if _, err := e.Begin("codex", "astra", false); err == nil {
		t.Fatal("missing input did not block")
	}
	e.State.Context["functional_request"] = "feature"
	e.State.Context["acceptance_criteria"] = "works"
	if _, err := e.Begin("codex", "astra", false); err != nil {
		t.Fatal(err)
	}
	e.Finish(0, false)
	if e.Condition("analysis_completed") {
		t.Fatal("exit code granted acceptance")
	}
	if err := e.Advance(); err == nil {
		t.Fatal("advanced before acceptance")
	}
	if err := e.Accept(); err != nil {
		t.Fatal(err)
	}
	if e.State.CurrentStep != "analysis" {
		t.Fatal("automatic transition")
	}
	if err := e.Advance(); err != nil {
		t.Fatal(err)
	}
	if err := e.Attest("design_validated_by_human", "yes", true); err == nil {
		t.Fatal("forged completion")
	}
	if _, err := e.Begin("codex", "astra", false); err != nil {
		t.Fatal(err)
	}
	e.Finish(0, false)
	if e.Condition("design_validated_by_human") {
		t.Fatal("design accepted automatically")
	}
}
func TestOptionalCheckpoint(t *testing.T) {
	e := engine(t, "feature", "backend_check")
	e.State.Step("backend").Status = "skipped"
	if err := e.Skip(); err != nil {
		t.Fatal(err)
	}
	if !e.Condition("backend_check_completed_when_applicable") || e.Condition("backend_tasks_completed") {
		t.Fatal("skip/completion semantics")
	}
	e.State.Step("backend_check").Status = "pending"
	e.State.Step("backend").Status = "completed"
	if err := e.Skip(); err == nil {
		t.Fatal("skipped required backend check")
	}
}
func TestTaskSessionsAndFallback(t *testing.T) {
	e := engine(t, "debug", "implementation")
	e.State.Context["bug_report"] = "bug"
	e.State.Step("plan").Status = "completed"
	ss := e.State.Step("implementation")
	ss.Tasks = []state.Task{{ID: "1", Text: "one", Status: "pending"}, {ID: "2", Text: "two", Status: "pending"}}
	if _, err := e.Begin("codex", "astra", true); err == nil {
		t.Fatal("fallback without failed or rejected attempt")
	}
	a, err := e.Begin("opencode", "default", false)
	if err != nil || a.TaskID != "1" {
		t.Fatalf("%v %v", a, err)
	}
	e.Finish(1, false)
	if err = e.Accept(); err == nil {
		t.Fatal("accepted failed result")
	}
	a, err = e.Begin("codex", "astra", true)
	if err != nil || a.Role != "expert" {
		t.Fatalf("%v %v", a, err)
	}
	e.Finish(0, false)
	if err = e.Accept(); err != nil {
		t.Fatal(err)
	}
	if ss.Status == "completed" || ActiveTask(ss).ID != "2" {
		t.Fatal("finished step before second task")
	}
	a, err = e.Begin("opencode", "default", false)
	if err != nil || a.TaskID != "2" || len(ss.Attempts) != 3 {
		t.Fatal("new task attempt missing")
	}
	e.Finish(-1, true)
	if e.State.Status != "interrupted" {
		t.Fatal("lost interruption")
	}
}
func TestRewindInvalidatesProgress(t *testing.T) {
	e := engine(t, "debug", "validation")
	e.State.Step("plan").Status = "completed"
	e.State.Step("review").Status = "completed"
	e.State.Conditions["related_tests_passed"] = state.Attestation{Value: true}
	if err := e.Rewind("plan"); err != nil {
		t.Fatal(err)
	}
	if e.Condition("review_completed") || e.Condition("related_tests_passed") || e.Condition("correction_plan_completed") {
		t.Fatal("stale validation survived rewind")
	}
}

func TestCorrectionConditionsAndCheckpoint(t *testing.T) {
	e := engine(t, "debug", "corrections")
	e.State.Context["bug_report"] = "report"
	e.State.Step("review").Status = "completed"
	if err := e.Attest("blocking_or_major_findings", "accepted review contains a major bug", true); err != nil {
		t.Fatal(err)
	}
	if err := e.Skip(); err == nil {
		t.Fatal("skipped known blocking/major findings")
	}
	e.State.Conditions["related_tests_passed"] = state.Attestation{Value: true}
	e.State.Step("corrections").Tasks = []state.Task{{ID: "fix", Text: "fix finding", Status: "pending"}}
	if _, err := e.Begin("opencode", "default", false); err != nil {
		t.Fatal(err)
	}
	if e.Condition("related_tests_passed") {
		t.Fatal("kept stale tests")
	}
	if !e.Condition("blocking_or_major_findings") {
		t.Fatal("lost historical review findings")
	}
	e.Finish(0, false)
	if err := e.Accept(); err != nil {
		t.Fatal(err)
	}
	if err := e.Advance(); err != nil {
		t.Fatal(err)
	}
	if err := e.Skip(); err == nil {
		t.Fatal("skipped required correction review")
	}

	f := engine(t, "feature", "backend_check")
	f.State.Context["functional_request"] = "request"
	f.State.Context["acceptance_criteria"] = "criteria"
	f.State.Step("backend").Status = "completed"
	if err := f.Accept(); err == nil {
		t.Fatal("accepted checkpoint without tests/manual validation")
	}
	for _, key := range []string{"related_tests_passed", "manual_backend_validation"} {
		if err := f.Attest(key, "checked", true); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Accept(); err != nil {
		t.Fatal(err)
	}
}
