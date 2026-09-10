// Package state persists execution data outside application repositories.
package state

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/S1933/agentic-workflow/internal/config"
)

type Decision struct {
	At     time.Time `json:"at"`
	Action string    `json:"action"`
	Step   string    `json:"step"`
	Detail string    `json:"detail"`
}
type Attestation struct {
	Value    bool      `json:"value"`
	Evidence string    `json:"evidence"`
	At       time.Time `json:"at"`
}
type Task struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}
type Step struct {
	Status   string    `json:"status"`
	Tasks    []Task    `json:"tasks,omitempty"`
	Feedback string    `json:"feedback,omitempty"`
	Attempts []Attempt `json:"attempts,omitempty"`
}
type Attempt struct {
	ID       string    `json:"id"`
	TaskID   string    `json:"task_id,omitempty"`
	Runtime  string    `json:"runtime"`
	Model    string    `json:"model"`
	Role     string    `json:"role"`
	Status   string    `json:"status"`
	Started  time.Time `json:"started"`
	ExitCode int       `json:"exit_code"`
}
type State struct {
	Version     int                    `json:"version"`
	RunID       string                 `json:"run_id"`
	Workflow    string                 `json:"workflow"`
	ProjectRoot string                 `json:"project_root"`
	Cwd         string                 `json:"cwd"`
	CurrentStep string                 `json:"current_step"`
	Status      string                 `json:"status"`
	Steps       map[string]*Step       `json:"steps"`
	Context     map[string]string      `json:"context"`
	Conditions  map[string]Attestation `json:"conditions"`
	Decisions   []Decision             `json:"decisions"`
	Created     time.Time              `json:"created"`
}

func (s *State) Record(action, detail string) {
	s.Decisions = append(s.Decisions, Decision{time.Now().UTC(), action, s.CurrentStep, detail})
}
func (s *State) Step(name string) *Step {
	if s.Steps[name] == nil {
		s.Steps[name] = &Step{Status: "pending"}
	}
	return s.Steps[name]
}

type Store struct{ Dir, Root string }

func Project(cwd string) (string, string, error) {
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", "", err
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", "", err
	}
	root := cwd
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	if data, err := cmd.Output(); err == nil {
		root = strings.TrimSpace(string(data))
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return "", "", err
		}
	}
	return root, cwd, nil
}
func Open(base, cwd string) (*Store, error) {
	root, _, err := Project(cwd)
	if err != nil {
		return nil, err
	}
	if base == "" {
		base = os.Getenv("XDG_STATE_HOME")
		if base != "" && !filepath.IsAbs(base) {
			return nil, fmt.Errorf("XDG_STATE_HOME must be absolute")
		}
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			base = filepath.Join(home, ".local", "state")
		}
		base = filepath.Join(base, "agentic-workflow")
	}
	base, err = filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	// Resolve symlinked parents even if the final directory does not exist yet.
	resolved, err := resolveFuture(base)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return nil, err
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return nil, fmt.Errorf("state directory must be outside target project")
	}
	digest := sha256.Sum256([]byte(root))
	return &Store{Dir: filepath.Join(resolved, "projects", hex.EncodeToString(digest[:])), Root: root}, nil
}
func resolveFuture(path string) (string, error) {
	if _, err := os.Lstat(path); err == nil {
		return filepath.EvalSymlinks(path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	parent, err := resolveFuture(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

// Lock uses an OS lock: process crashes release it without stale lock cleanup.
// The MVP supports macOS and Linux.
func (st *Store) Lock() (func(), error) {
	if err := os.MkdirAll(st.Dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(st.Dir, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another agentic process owns this project: %w", err)
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
func (st *Store) Load() (*State, error) {
	data, err := os.ReadFile(filepath.Join(st.Dir, "state.json"))
	if err != nil {
		return nil, err
	}
	var s State
	if err = json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Version != 1 || s.ProjectRoot != st.Root || len(s.RunID) != 32 || s.Steps == nil || s.Context == nil || s.Conditions == nil {
		return nil, fmt.Errorf("invalid or unsupported saved state")
	}
	if _, err := hex.DecodeString(s.RunID); err != nil {
		return nil, fmt.Errorf("invalid run identifier")
	}
	return &s, nil
}
func (st *Store) RunDir(s *State) string { return filepath.Join(st.Dir, "runs", s.RunID) }
func (st *Store) Create(c *config.Config, workflow, cwd string) (*State, error) {
	if _, err := os.Stat(filepath.Join(st.Dir, "state.json")); err == nil {
		return nil, fmt.Errorf("a workflow already exists; use resume or reset")
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	w, ok := c.Workflow.Workflows[workflow]
	if !ok {
		return nil, fmt.Errorf("unknown workflow %s", workflow)
	}
	root, canonical, err := Project(cwd)
	if err != nil {
		return nil, err
	}
	if root != st.Root {
		return nil, fmt.Errorf("project changed")
	}
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return nil, err
	}
	s := &State{Version: 1, RunID: hex.EncodeToString(id), Workflow: workflow, ProjectRoot: root, Cwd: canonical, CurrentStep: w.Ordered()[0].Name, Status: "ready", Steps: map[string]*Step{}, Context: map[string]string{}, Conditions: map[string]Attestation{}, Created: time.Now().UTC()}
	dir := st.RunDir(s)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err = Atomic(filepath.Join(dir, "workflow.yaml"), c.WorkflowBytes); err != nil {
		return nil, err
	}
	if err = Atomic(filepath.Join(dir, "runtime.yml"), c.RuntimeBytes); err != nil {
		return nil, err
	}
	if err = st.Save(s); err != nil {
		return nil, err
	}
	return s, nil
}
func (st *Store) Save(s *State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return Atomic(filepath.Join(st.Dir, "state.json"), append(b, '\n'))
}
func (st *Store) Archive(s *State) error {
	return os.Rename(filepath.Join(st.Dir, "state.json"), filepath.Join(st.RunDir(s), "archived-state.json"))
}
func Atomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".agentic-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
