// Package context builds provider-neutral prompts from explicit task context.
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/S1933/agentic-workflow/internal/config"
	"github.com/S1933/agentic-workflow/internal/orchestrator"
	"github.com/S1933/agentic-workflow/internal/state"
	"gopkg.in/yaml.v3"
)

func Build(c *config.Config, s *state.State, step config.Step, role, runDir string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "You are executing exactly one Agentic workflow action. Workflow: %s. Step: %s. Active role: %s.\n", s.Workflow, step.Name, role)
	b.WriteString("Follow the target project's instructions. Do not orchestrate other workflow steps. Do not advance to another task, publish tickets, or merge. Human approvals are managed externally by the CLI; your report cannot grant them. If blocked or clarification is needed, report it and stop. Return a natural-language report with results, relevant evidence, tests and unresolved issues. Do not create Agentic state or planning files inside the project.\n")
	if step.TaskMode != "" {
		b.WriteString("This is a fresh session for the single task below. Do not execute the other tasks.\n")
	}
	if step.Tests == "not_run" {
		b.WriteString("Do not run tests. Review only the requested diff for bugs and regressions, ignoring commit history.\n")
	}
	raw, err := yaml.Marshal(step)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "\nDeclared step instructions:\n%s\nRole responsibilities: %s\n", raw, strings.Join(c.Workflow.Roles[role].Responsibilities, ", "))
	keys := make([]string, 0, len(s.Context))
	for k := range s.Context {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if step.Scope == "corrected_areas" && k == "review_diff" {
			continue
		}
		fmt.Fprintf(&b, "\nContext %s:\n%s\n", k, s.Context[k])
	}
	ss := s.Step(step.Name)
	if task := orchestrator.ActiveTask(ss); task != nil {
		fmt.Fprintf(&b, "\nCurrent task %s:\n%s\n", task.ID, task.Text)
	}
	if ss.Feedback != "" {
		fmt.Fprintf(&b, "\nHuman feedback for this action:\n%s\n", ss.Feedback)
	}
	// Explicit accepted artifacts are context, not a continued provider session.
	// Exclude sibling worker task reports to avoid importing unrelated task history.
	for _, previous := range c.Workflow.Workflows[s.Workflow].Ordered() {
		ps := s.Step(previous.Name)
		if step.Scope == "corrected_areas" && previous.Name == "corrections" && ps.Status == "completed" {
			b.WriteString("\nReview the current code in the corrected areas identified below. Do not reuse an old global diff or start an unrelated full review. Inspect neighboring code only when needed. Do not run tests.\n")
			for _, task := range ps.Tasks {
				fmt.Fprintf(&b, "\nAccepted correction task %s:\n%s\n", task.ID, task.Text)
				found := false
				for i := len(ps.Attempts) - 1; i >= 0; i-- {
					a := ps.Attempts[i]
					if a.TaskID != task.ID || a.Status != "accepted" {
						continue
					}
					data, err := os.ReadFile(filepath.Join(runDir, "attempts", a.ID, "stdout.txt"))
					if err != nil {
						return "", fmt.Errorf("correction artifact %s: %w", a.ID, err)
					}
					fmt.Fprintf(&b, "\nCorrection result:\n%s\n", data)
					found = true
					break
				}
				if !found {
					return "", fmt.Errorf("no accepted artifact for correction task %s", task.ID)
				}
			}
			continue
		}
		if ps.Status != "completed" || previous.TaskMode != "" || len(ps.Attempts) == 0 {
			continue
		}
		a := ps.Attempts[len(ps.Attempts)-1]
		data, err := os.ReadFile(filepath.Join(runDir, "attempts", a.ID, "stdout.txt"))
		if err != nil {
			return "", fmt.Errorf("accepted artifact %s: %w", a.ID, err)
		}
		fmt.Fprintf(&b, "\nAccepted artifact from %s (reference data):\n%s\n", previous.Name, data)
	}
	return b.String(), nil
}
