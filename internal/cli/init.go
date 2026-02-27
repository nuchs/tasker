package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nuchs/tasker/internal/store"
)

// RunInit implements the `tracker init` subcommand.
// wd is the working directory in which to create .tracker/; args are the
// remaining command-line arguments after "init".
//
// Running init twice in the same directory is an error — if .tracker/ already
// exists the command fails immediately without modifying any files.
func RunInit(wd string, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	prefix := fs.String("prefix", "", "project prefix for issue IDs (e.g. PROJ)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *prefix == "" {
		return fmt.Errorf("init: --prefix is required")
	}

	trackerDir := filepath.Join(wd, ".tracker")
	if _, err := os.Stat(trackerDir); err == nil {
		return fmt.Errorf("init: already initialised (%s exists)", trackerDir)
	}

	issuesDir := filepath.Join(trackerDir, "issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		return fmt.Errorf("init: create issues dir: %w", err)
	}

	if err := writeConfig(Config{Prefix: *prefix}, filepath.Join(trackerDir, "config.yaml")); err != nil {
		return fmt.Errorf("init: %w", err)
	}

	if err := os.WriteFile(filepath.Join(trackerDir, ".gitignore"), []byte("db.sqlite\n"), 0644); err != nil {
		return fmt.Errorf("init: write .gitignore: %w", err)
	}

	s, err := store.Open(issuesDir, filepath.Join(trackerDir, "db.sqlite"), *prefix)
	if err != nil {
		return fmt.Errorf("init: create database: %w", err)
	}
	s.Close()

	fmt.Printf("Initialised tracker with prefix %s in %s\n", *prefix, trackerDir)
	return nil
}
