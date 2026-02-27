package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/nuchs/tasker/internal/cli"
)

func TestRunInit_CreatesExpectedStructure(t *testing.T) {
	wd := t.TempDir()

	if err := cli.RunInit(wd, []string{"--prefix", "PROJ"}); err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	trackerDir := filepath.Join(wd, ".tracker")

	// .tracker/ exists
	if _, err := os.Stat(trackerDir); err != nil {
		t.Errorf(".tracker/ missing: %v", err)
	}

	// .tracker/issues/ exists
	if _, err := os.Stat(filepath.Join(trackerDir, "issues")); err != nil {
		t.Errorf(".tracker/issues/ missing: %v", err)
	}

	// .tracker/config.yaml exists and contains the prefix
	cfgData, err := os.ReadFile(filepath.Join(trackerDir, "config.yaml"))
	if err != nil {
		t.Fatalf("config.yaml missing: %v", err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(cfgData, &cfg); err != nil {
		t.Fatalf("parse config.yaml: %v", err)
	}
	if cfg["prefix"] != "PROJ" {
		t.Errorf("config prefix: got %v, want PROJ", cfg["prefix"])
	}

	// .tracker/.gitignore contains db.sqlite
	gi, err := os.ReadFile(filepath.Join(trackerDir, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore missing: %v", err)
	}
	if !strings.Contains(string(gi), "db.sqlite") {
		t.Errorf(".gitignore does not contain db.sqlite: %q", string(gi))
	}

	// .tracker/db.sqlite exists
	if _, err := os.Stat(filepath.Join(trackerDir, "db.sqlite")); err != nil {
		t.Errorf("db.sqlite missing: %v", err)
	}
}

func TestRunInit_RequiresPrefix(t *testing.T) {
	wd := t.TempDir()
	err := cli.RunInit(wd, []string{})
	if err == nil {
		t.Fatal("expected error when --prefix is missing, got nil")
	}
}

func TestRunInit_TwiceIsError(t *testing.T) {
	wd := t.TempDir()

	if err := cli.RunInit(wd, []string{"--prefix", "PROJ"}); err != nil {
		t.Fatalf("first RunInit: %v", err)
	}
	if err := cli.RunInit(wd, []string{"--prefix", "PROJ"}); err == nil {
		t.Fatal("expected error on second RunInit, got nil")
	}
}

func TestRunInit_DatabaseHasSchema(t *testing.T) {
	wd := t.TempDir()

	if err := cli.RunInit(wd, []string{"--prefix", "T"}); err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	// Opening the Store verifies the database exists and schema was created.
	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore (database schema not created correctly): %v", err)
	}
	s.Close()
}

func TestFindTrackerDir_FindsInCurrent(t *testing.T) {
	wd := t.TempDir()
	cli.RunInit(wd, []string{"--prefix", "X"}) //nolint:errcheck

	got, err := cli.FindTrackerDir(wd)
	if err != nil {
		t.Fatalf("FindTrackerDir: %v", err)
	}
	want := filepath.Join(wd, ".tracker")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFindTrackerDir_FindsInParent(t *testing.T) {
	root := t.TempDir()
	cli.RunInit(root, []string{"--prefix", "X"}) //nolint:errcheck

	subdir := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	got, err := cli.FindTrackerDir(subdir)
	if err != nil {
		t.Fatalf("FindTrackerDir: %v", err)
	}
	want := filepath.Join(root, ".tracker")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFindTrackerDir_ErrorWhenNotFound(t *testing.T) {
	dir := t.TempDir()
	if _, err := cli.FindTrackerDir(dir); err == nil {
		t.Fatal("expected error when no .tracker/ found, got nil")
	}
}
