package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/S1933/agentic-workflow/internal/config"
	"github.com/S1933/agentic-workflow/internal/state"
)

func TestExplicitTaskContext(t *testing.T) {
	c, err := config.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	s := &state.State{Workflow: "feature", CurrentStep: "frontend", Context: map[string]string{"spec": "published specification"}, Steps: map[string]*state.Step{}}
	s.Step("frontend").Tasks = []state.Task{{ID: "a", Text: "old task", Status: "completed"}, {ID: "b", Text: "current objective", Status: "pending"}}
	s.Step("design").Status = "completed"
	s.Step("design").Attempts = []state.Attempt{{ID: "design-0001"}}
	s.Step("backend").Status = "completed"
	s.Step("backend").Attempts = []state.Attempt{{ID: "backend-0001"}}
	dir := t.TempDir()
	path := filepath.Join(dir, "attempts", "design-0001")
	if err = os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(path, "stdout.txt"), []byte("accepted design"), 0600); err != nil {
		t.Fatal(err)
	}
	var step config.Step
	for _, candidate := range c.Workflow.Workflows["feature"].Ordered() {
		if candidate.Name == "frontend" {
			step = candidate
		}
	}
	prompt, err := Build(c, s, step, "worker", dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"published specification", "accepted design", "current objective", "fresh session"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("missing %q", required)
		}
	}
	if strings.Contains(prompt, "old task") {
		t.Fatal("previous task leaked into prompt")
	}
}

func TestCorrectionReviewContext(t *testing.T) {
	c, err := config.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	s := &state.State{Workflow: "debug", CurrentStep: "correction_review", Context: map[string]string{"review_diff": "STALE GLOBAL DIFF"}, Steps: map[string]*state.Step{}}
	ss := s.Step("corrections")
	ss.Status = "completed"
	ss.Tasks = []state.Task{{ID: "one", Text: "fix timeout", Status: "completed"}, {ID: "two", Text: "fix cache", Status: "completed"}}
	ss.Attempts = []state.Attempt{{ID: "corrections-0001", TaskID: "one", Status: "accepted"}, {ID: "corrections-0002", TaskID: "two", Status: "failed"}, {ID: "corrections-0003", TaskID: "two", Status: "accepted"}}
	dir := t.TempDir()
	for _, id := range []string{"corrections-0001", "corrections-0003"} {
		path := filepath.Join(dir, "attempts", id)
		if err = os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(path, "stdout.txt"), []byte("accepted report "+id), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var step config.Step
	for _, candidate := range c.Workflow.Workflows["debug"].Ordered() {
		if candidate.Name == "correction_review" {
			step = candidate
		}
	}
	prompt, err := Build(c, s, step, "expert", dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"fix timeout", "fix cache", "accepted report corrections-0001", "accepted report corrections-0003"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("missing %q", required)
		}
	}
	if strings.Contains(prompt, "STALE GLOBAL DIFF") {
		t.Fatal("stale full diff included in targeted review")
	}
}
