// Package config loads the provider-independent workflow and runtime bindings.
package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

type Condition struct {
	Kind                  string   `yaml:"kind"`
	Step                  string   `yaml:"step,omitempty"`
	Steps                 []string `yaml:"steps,omitempty"`
	AllowSkipped          bool     `yaml:"allow_skipped,omitempty"`
	After                 string   `yaml:"after,omitempty"`
	InvalidateOnExecution bool     `yaml:"invalidate_on_execution,omitempty"`
}
type Step struct {
	Name              string            `yaml:"-"`
	Role              string            `yaml:"role,omitempty"`
	Optional          bool              `yaml:"optional,omitempty"`
	When              string            `yaml:"when,omitempty"`
	Requires          []string          `yaml:"requires,omitempty"`
	Validation        string            `yaml:"validation,omitempty"`
	Mode              string            `yaml:"mode,omitempty"`
	TaskMode          string            `yaml:"task_mode,omitempty"`
	Session           string            `yaml:"session,omitempty"`
	Fallback          *Fallback         `yaml:"fallback,omitempty"`
	Context           []string          `yaml:"context,omitempty"`
	Inspect           []string          `yaml:"inspect,omitempty"`
	Tests             string            `yaml:"tests,omitempty"`
	SourceOfTruth     string            `yaml:"source_of_truth,omitempty"`
	Trigger           string            `yaml:"trigger,omitempty"`
	Input             map[string]string `yaml:"input,omitempty"`
	Focus             []string          `yaml:"focus,omitempty"`
	Findings          map[string]string `yaml:"findings,omitempty"`
	Priority          []string          `yaml:"priority,omitempty"`
	Scope             string            `yaml:"scope,omitempty"`
	ExpandScope       string            `yaml:"expand_scope,omitempty"`
	BeforeCodeChanges bool              `yaml:"before_code_changes,omitempty"`
	RootCause         string            `yaml:"root_cause,omitempty"`
	Evidence          string            `yaml:"evidence,omitempty"`
	Confidence        string            `yaml:"confidence,omitempty"`
	RegressionTest    string            `yaml:"regression_test,omitempty"`
}
type Fallback struct {
	Role     string   `yaml:"role"`
	Requires []string `yaml:"requires"`
	Session  string   `yaml:"session"`
}
type Workflow struct {
	Requires []string          `yaml:"requires"`
	Steps    []map[string]Step `yaml:"steps"`
}

func (w Workflow) Ordered() []Step {
	out := make([]Step, 0, len(w.Steps))
	for _, item := range w.Steps {
		for name, step := range item {
			step.Name = name
			out = append(out, step)
		}
	}
	return out
}

type WorkflowFile struct {
	Version        int `yaml:"version"`
	AgentExecution struct {
		SourceOfTruth     string `yaml:"source_of_truth"`
		WorkflowSelection map[string]struct {
			When []string `yaml:"when"`
		} `yaml:"workflow_selection"`
		Execution struct {
			StepOrder             string `yaml:"step_order"`
			RespectRequires       bool   `yaml:"respect_requires"`
			ExecuteActiveRoleOnly bool   `yaml:"execute_active_role_only"`
			OptionalSteps         string `yaml:"optional_steps"`
			HumanValidation       string `yaml:"human_validation"`
			ManualTransition      string `yaml:"manual_transition"`
			FallbackHumanDecision string `yaml:"fallback_human_decision"`
		} `yaml:"execution"`
		State struct {
			Required      bool     `yaml:"required"`
			SuggestedPath string   `yaml:"suggested_path"`
			Fields        []string `yaml:"fields"`
			Lifecycle     string   `yaml:"lifecycle"`
			Persistence   string   `yaml:"persistence"`
		} `yaml:"state"`
	} `yaml:"agent_execution"`
	Roles map[string]struct {
		Responsibilities []string `yaml:"responsibilities"`
	} `yaml:"roles"`
	Shared struct {
		Transitions     string `yaml:"transitions"`
		FinalValidation string `yaml:"final_validation"`
	} `yaml:"shared"`
	Conditions map[string]Condition `yaml:"conditions"`
	Workflows  map[string]Workflow  `yaml:"workflows"`
}
type Model struct {
	Model string `yaml:"model"`
}
type Binding struct {
	Selection string           `yaml:"selection"`
	Runtimes  map[string]Model `yaml:"runtimes"`
}
type RuntimeFile struct {
	Version int `yaml:"version"`
	Roles   map[string]struct {
		Capability string `yaml:"capability"`
	} `yaml:"roles"`
	Bindings map[string]Binding `yaml:"bindings"`
}
type Config struct {
	Workflow      WorkflowFile
	Runtime       RuntimeFile
	WorkflowBytes []byte
	RuntimeBytes  []byte
}

func decode(data []byte, target any) error {
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected exactly one YAML document")
	}
	return nil
}
func Load(dir string) (*Config, error) {
	w, err := os.ReadFile(filepath.Join(dir, "workflow.yaml"))
	if err != nil {
		return nil, err
	}
	r, err := os.ReadFile(filepath.Join(dir, "runtime.yml"))
	if err != nil {
		return nil, err
	}
	return Parse(w, r)
}
func Parse(w, r []byte) (*Config, error) {
	c := &Config{WorkflowBytes: w, RuntimeBytes: r}
	if err := decode(w, &c.Workflow); err != nil {
		return nil, fmt.Errorf("workflow.yaml: %w", err)
	}
	if err := decode(r, &c.Runtime); err != nil {
		return nil, fmt.Errorf("runtime.yml: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}
func (c *Config) Validate() error {
	if c.Workflow.Version != 1 || c.Runtime.Version != 1 {
		return fmt.Errorf("unsupported configuration version")
	}
	e := c.Workflow.AgentExecution.Execution
	policy := c.Workflow.AgentExecution.State
	if !policy.Required || policy.Persistence != "required" || policy.Lifecycle != "workflow" {
		return fmt.Errorf("MVP requires persistent workflow state")
	}
	if c.Workflow.Shared.Transitions != "manual" || c.Workflow.Shared.FinalValidation != "human" || e.StepOrder != "declared" || !e.RespectRequires || !e.ExecuteActiveRoleOnly || e.OptionalSteps != "conditional" || e.HumanValidation != "blocking" || e.ManualTransition != "stop_and_wait" || e.FallbackHumanDecision != "stop_and_wait" {
		return fmt.Errorf("unsupported execution policy: MVP requires declared order and blocking human transitions")
	}
	if len(c.Workflow.Workflows) == 0 {
		return fmt.Errorf("no workflows configured")
	}
	for role := range c.Workflow.Roles {
		if _, err := c.Candidates(role); err != nil {
			return err
		}
	}
	for name, w := range c.Workflow.Workflows {
		seen := map[string]bool{}
		positions := map[string]int{}
		if len(w.Steps) == 0 {
			return fmt.Errorf("workflow %s has no steps", name)
		}
		for _, item := range w.Steps {
			if len(item) != 1 {
				return fmt.Errorf("each step must have exactly one name")
			}
		}
		for i, s := range w.Ordered() {
			if !regexp.MustCompile(`^[a-z][a-z0-9_]*$`).MatchString(s.Name) {
				return fmt.Errorf("invalid step identifier %q", s.Name)
			}
			if seen[s.Name] {
				return fmt.Errorf("duplicate step %s", s.Name)
			}
			seen[s.Name] = true
			positions[s.Name] = i
			if s.Role == "" && s.Mode != "human" && s.Mode != "manual" {
				return fmt.Errorf("step %s needs a role or human mode", s.Name)
			}
			if s.Role != "" {
				if _, ok := c.Workflow.Roles[s.Role]; !ok {
					return fmt.Errorf("unknown role %s", s.Role)
				}
			}
			if s.Mode != "" && s.Mode != "human" && s.Mode != "manual" {
				return fmt.Errorf("unsupported mode %s", s.Mode)
			}
			if s.Validation != "" && s.Validation != "human" {
				return fmt.Errorf("unsupported validation %s", s.Validation)
			}
			if s.Trigger != "" && s.Trigger != "manual" {
				return fmt.Errorf("unsupported trigger %s", s.Trigger)
			}
			if s.TaskMode != "" && s.TaskMode != "one_at_a_time" {
				return fmt.Errorf("unsupported task mode %s", s.TaskMode)
			}
			if s.Session != "" && s.Session != "fresh" && s.Session != "fresh_per_task" {
				return fmt.Errorf("unsupported session %s", s.Session)
			}
			if s.TaskMode != "" && s.Session != "fresh_per_task" {
				return fmt.Errorf("task step %s needs fresh_per_task", s.Name)
			}
			if s.Fallback != nil {
				f := s.Fallback
				if _, ok := c.Workflow.Roles[f.Role]; !ok || f.Session != "fresh" || len(f.Requires) != 1 || f.Requires[0] != "human_decision" {
					return fmt.Errorf("invalid fallback for %s", s.Name)
				}
			}
		}
		used := append([]string{}, w.Requires...)
		for _, s := range w.Ordered() {
			used = append(used, s.Requires...)
			if s.When != "" {
				if !s.Optional {
					return fmt.Errorf("when requires optional step")
				}
				used = append(used, s.When)
			}
		}
		for _, key := range used {
			cond, ok := c.Workflow.Conditions[key]
			if !ok {
				return fmt.Errorf("undefined condition %s in %s", key, name)
			}
			switch cond.Kind {
			case "input":
			case "human":
				if cond.After != "" && !seen[cond.After] {
					return fmt.Errorf("unknown condition predecessor %s", cond.After)
				}
			case "completion":
				if !seen[cond.Step] {
					return fmt.Errorf("unknown condition step %s", cond.Step)
				}
			case "aggregate":
				if len(cond.Steps) == 0 {
					return fmt.Errorf("empty aggregate %s", key)
				}
				for _, step := range cond.Steps {
					if !seen[step] {
						return fmt.Errorf("unknown aggregate step %s", step)
					}
				}
			default:
				return fmt.Errorf("unsupported condition kind %s", cond.Kind)
			}
		}
		for _, step := range w.Ordered() {
			keys := append([]string{}, step.Requires...)
			if step.When != "" {
				keys = append(keys, step.When)
			}
			for _, key := range keys {
				condition := c.Workflow.Conditions[key]
				deps := append([]string{}, condition.Steps...)
				if condition.Step != "" {
					deps = append(deps, condition.Step)
				}
				if condition.After != "" {
					deps = append(deps, condition.After)
				}
				for _, dep := range deps {
					if positions[dep] >= positions[step.Name] {
						return fmt.Errorf("condition %s must depend on an earlier step than %s", key, step.Name)
					}
				}
			}
		}
	}
	return nil
}

type Candidate struct{ Name, Model string }

func (c *Config) Candidates(role string) ([]Candidate, error) {
	r, ok := c.Runtime.Roles[role]
	if !ok {
		return nil, fmt.Errorf("no runtime role %s", role)
	}
	b, ok := c.Runtime.Bindings[r.Capability]
	if !ok || len(b.Runtimes) == 0 {
		return nil, fmt.Errorf("no runtimes for %s", role)
	}
	if b.Selection != "human" {
		return nil, fmt.Errorf("unsupported selection %q", b.Selection)
	}
	out := []Candidate{}
	for name, m := range b.Runtimes {
		if m.Model == "" {
			return nil, fmt.Errorf("empty model for %s", name)
		}
		out = append(out, Candidate{name, m.Model})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
