package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const ConfigName = "gosqlkit.yaml"

const (
	DefaultMigrationsDir    = "db/migrations"
	DefaultMigrationsRunner = "goose"
)

type Config struct {
	Out        OutSpec       `json:"out" yaml:"out"`
	Migrations MigrationSpec `json:"migrations" yaml:"migrations"`
	Version    string        `json:"version" yaml:"version"`
	Dialect    string        `json:"dialect" yaml:"dialect"`
	rootDir    string
	Schema     SchemaSpec `json:"schema" yaml:"schema"`
}

type SchemaSpec struct {
	paths []string
}

type OutSpec struct {
	SQL      string `json:"sql" yaml:"sql"`
	Snapshot string `json:"snapshot" yaml:"snapshot"`
}

type MigrationSpec struct {
	Dir    string `json:"dir" yaml:"dir"`
	Runner string `json:"runner" yaml:"runner"`
}

func (s *SchemaSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var single string
		if err := value.Decode(&single); err != nil {
			return err
		}
		s.paths = []string{single}
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		var multi []string
		if err := value.Decode(&multi); err != nil {
			return err
		}
		s.paths = multi
		return nil
	}
	return fmt.Errorf("schema must be a string or list of strings, got kind %d", value.Kind)
}

func (s SchemaSpec) Paths() []string {
	return s.paths
}

func (c *Config) RootDir() string {
	return c.rootDir
}

func (c *Config) ResolvePath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.rootDir, path)
}

func LoadConfig(path string) (*Config, error) {
	// #nosec G304 -- path is the user-selected config file, resolved from the CLI or discovery.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	if len(config.Schema.paths) == 0 {
		return nil, fmt.Errorf("config %q must specify at least one schema path", path)
	}
	if config.Dialect == "" {
		config.Dialect = "postgres"
	}
	if config.Migrations.Dir == "" {
		config.Migrations.Dir = DefaultMigrationsDir
	}
	config.Migrations.Runner = strings.TrimSpace(strings.ToLower(config.Migrations.Runner))
	if config.Migrations.Runner == "" {
		config.Migrations.Runner = DefaultMigrationsRunner
	}
	if config.Migrations.Runner != DefaultMigrationsRunner {
		return nil, fmt.Errorf("config %q has unsupported migrations.runner %q; supported values: %s",
			path, config.Migrations.Runner, DefaultMigrationsRunner)
	}

	config.rootDir = filepath.Dir(path)
	return &config, nil
}

func DiscoverConfig(root string) (*Config, error) {
	dir := root
	for {
		candidate := filepath.Join(dir, ConfigName)
		if _, err := os.Stat(candidate); err == nil {
			return LoadConfig(candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no %s found in %s or any parent directory", ConfigName, root)
		}
		dir = parent
	}
}

func (c *Config) SchemaPaths() []string {
	paths := make([]string, 0, len(c.Schema.paths))
	for _, p := range c.Schema.paths {
		paths = append(paths, c.ResolvePath(p))
	}
	return paths
}

func (c *Config) SQLPath() string {
	return c.ResolvePath(c.Out.SQL)
}

func (c *Config) SnapshotPath() string {
	return c.ResolvePath(c.Out.Snapshot)
}

func (c *Config) MigrationsDir() string {
	return c.ResolvePath(c.Migrations.Dir)
}

func (c *Config) String() string {
	return fmt.Sprintf("config{dialect=%s schema=%s out.sql=%s out.snapshot=%s migrations.dir=%s migrations.runner=%s}",
		c.Dialect, strings.Join(c.Schema.paths, ", "), c.Out.SQL, c.Out.Snapshot,
		c.Migrations.Dir, c.Migrations.Runner)
}
