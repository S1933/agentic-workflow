package state

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/S1933/agentic-workflow/internal/config"
)

func TestPersistenceAndIsolation(t *testing.T) {
	base := t.TempDir()
	project := t.TempDir()
	store, err := Open(base, project)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := store.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err = store.Lock(); err == nil {
		t.Fatal("concurrent lock accepted")
	}
	c, err := config.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Create(c, "debug", project)
	if err != nil {
		t.Fatal(err)
	}
	s.Context["bug_report"] = "failure"
	if err = store.Save(s); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil || loaded.Context["bug_report"] != "failure" {
		t.Fatalf("%v %v", loaded, err)
	}
	if _, err = store.Create(c, "feature", project); err == nil {
		t.Fatal("overwrote active run")
	}
	other, err := Open(base, t.TempDir())
	if err != nil || other.Dir == store.Dir {
		t.Fatal("project identity collision")
	}
	entries, _ := os.ReadDir(project)
	if len(entries) != 0 {
		t.Fatal("wrote into target")
	}
	if err = store.Archive(s); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("state still active: %v", err)
	}
	if _, err = os.Stat(filepath.Join(store.RunDir(s), "archived-state.json")); err != nil {
		t.Fatal(err)
	}
}
func TestRejectStateInsideProject(t *testing.T) {
	project := t.TempDir()
	if _, err := Open(filepath.Join(project, "state"), project); err == nil {
		t.Fatal("accepted project-local state")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(project, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(link, "nested", "state"), project); err == nil {
		t.Fatal("accepted symlinked project state")
	}
}

func TestGitProjectIdentity(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("git", "init", "--quiet", root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, output)
	}
	sub := filepath.Join(root, "nested")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	a, err := Open(base, root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(base, sub)
	if err != nil {
		t.Fatal(err)
	}
	if a.Dir != b.Dir {
		t.Fatal("nested directory created another project")
	}
	_, cwd, err := Project(sub)
	if err != nil || filepath.Base(cwd) != "nested" {
		t.Fatal("lost invocation cwd")
	}
	link := filepath.Join(t.TempDir(), "alias")
	if err = os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	c, err := Open(base, link)
	if err != nil || c.Dir != a.Dir {
		t.Fatal("symlink changed project identity")
	}
}
