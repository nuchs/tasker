package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/nuchs/tasker/internal/store"
)

// Config holds the persistent project configuration stored in .tracker/config.yaml.
type Config struct {
	Prefix string `yaml:"prefix"`
}

// LoadConfig reads and parses .tracker/config.yaml from trackerDir.
func LoadConfig(trackerDir string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(trackerDir, "config.yaml"))
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// writeConfig serialises cfg to path.
func writeConfig(cfg Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// FindTrackerDir walks up from startDir looking for a .tracker/ directory.
// Returns the full path to .tracker/ or an error if none is found.
func FindTrackerDir(startDir string) (string, error) {
	dir := startDir
	for {
		candidate := filepath.Join(dir, ".tracker")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a tracker repository (no .tracker/ found)")
		}
		dir = parent
	}
}

// OpenStore finds the tracker directory from startDir, loads the config, and
// opens a Store. It is the standard way CLI commands obtain a Store.
func OpenStore(startDir string) (*store.Store, error) {
	trackerDir, err := FindTrackerDir(startDir)
	if err != nil {
		return nil, err
	}
	cfg, err := LoadConfig(trackerDir)
	if err != nil {
		return nil, err
	}
	return store.Open(
		filepath.Join(trackerDir, "issues"),
		filepath.Join(trackerDir, "db.sqlite"),
		cfg.Prefix,
	)
}
