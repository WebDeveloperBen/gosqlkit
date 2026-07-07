package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pgplan "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

type MigrateCreateOptions struct {
	CreatedAt time.Time
	Name      string
	Dir       string
	Runner    string
	Empty     bool
	NoDown    bool
}

type MigrateCreateResult struct {
	Dir   string   `json:"dir"`
	Files []string `json:"files"`
}

type MigrateCheckOptions struct {
	Dir string
}

type MigrateCheckResult struct {
	Dir   string `json:"dir"`
	Count int    `json:"count"`
}

func MigrateCreateWithConfig(config *Config, opts MigrateCreateOptions) (*MigrateCreateResult, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if opts.Name == "" {
		return nil, errors.New("migration name is required")
	}

	runner := opts.Runner
	if runner == "" {
		runner = config.Migrations.Runner
	}

	dir := opts.Dir
	if dir == "" {
		dir = config.MigrationsDir()
	} else {
		dir = config.ResolvePath(dir)
	}

	downSQL := []string{}
	if opts.NoDown {
		downSQL = nil
	}

	plan := migrate.Plan{
		Name:      opts.Name,
		Dialect:   config.Dialect,
		CreatedAt: opts.CreatedAt,
		Changes:   []migrate.Change{},
		UpSQL:     []string{},
		DownSQL:   downSQL,
	}

	if !opts.Empty {
		planned, err := plannedMigration(config, dir, opts)
		if err != nil {
			return nil, err
		}
		plan = planned
	}

	files, err := renderMigrationFiles(runner, plan)
	if err != nil {
		return nil, err
	}

	written := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(dir, file.Name)
		if err := writeNewFile(path, file.Content); err != nil {
			return nil, err
		}
		written = append(written, path)
	}

	return &MigrateCreateResult{Dir: dir, Files: written}, nil
}

func plannedMigration(config *Config, dir string, opts MigrateCreateOptions) (migrate.Plan, error) {
	existing, err := migrate.ScanDir(dir)
	if err != nil {
		return migrate.Plan{}, err
	}
	if len(existing) > 0 && len(existing[len(existing)-1].Metadata.TargetSnapshot) == 0 {
		return migrate.Plan{}, errors.New("latest migration has no targetSnapshot metadata; cannot create diff migration")
	}

	sql, _, err := renderSQLWithConfig(config)
	if err != nil {
		return migrate.Plan{}, err
	}
	snapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		return migrate.Plan{}, err
	}
	snapshotID, err := snapshotIDFromJSON(snapshot)
	if err != nil {
		return migrate.Plan{}, err
	}

	if len(existing) == 0 {
		return baselineMigrationPlan(config, opts, sql, snapshot, snapshotID), nil
	}
	return diffMigrationPlan(config, opts, existing[len(existing)-1], snapshot, snapshotID)
}

func baselineMigrationPlan(config *Config, opts MigrateCreateOptions, sql, snapshot, snapshotID string) migrate.Plan {
	return migrate.Plan{
		Name:           opts.Name,
		Dialect:        config.Dialect,
		CreatedAt:      opts.CreatedAt,
		ToSnapshotID:   snapshotID,
		TargetSnapshot: snapshot,
		Changes: []migrate.Change{{
			Op:      "baseline",
			Object:  "schema",
			Summary: "Create baseline schema from current gosqlkit definitions",
		}},
		UpSQL:   []string{sql},
		DownSQL: nil,
	}
}

func diffMigrationPlan(config *Config, opts MigrateCreateOptions, previous migrate.Migration, snapshot, snapshotID string) (migrate.Plan, error) {
	planner, err := snapshotPlanner(config.Dialect)
	if err != nil {
		return migrate.Plan{}, err
	}
	planned, err := planner.PlanSnapshotDiff(previous.Metadata.TargetSnapshot, []byte(snapshot))
	if err != nil {
		return migrate.Plan{}, err
	}
	if len(planned.Statements) == 0 {
		return migrate.Plan{}, errors.New("schema has no changes")
	}

	changes := make([]migrate.Change, 0, len(planned.Changes))
	for _, change := range planned.Changes {
		risks := make([]string, 0, len(change.Risks))
		for _, risk := range change.Risks {
			risks = append(risks, string(risk))
		}
		changes = append(changes, migrate.Change{
			Op:      string(change.Op),
			Object:  change.Object,
			Summary: change.Summary,
			Risks:   risks,
		})
	}

	return migrate.Plan{
		Name:           opts.Name,
		Dialect:        config.Dialect,
		FromSnapshotID: previous.Metadata.ToSnapshotID,
		ToSnapshotID:   snapshotID,
		TargetSnapshot: snapshot,
		CreatedAt:      opts.CreatedAt,
		Changes:        changes,
		UpSQL:          planned.Statements,
		DownSQL:        nil,
	}, nil
}

func MigrateCheckWithConfig(config *Config, opts MigrateCheckOptions) (*MigrateCheckResult, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}

	dir := opts.Dir
	if dir == "" {
		dir = config.MigrationsDir()
	} else {
		dir = config.ResolvePath(dir)
	}

	migrations, err := migrate.ScanDir(dir)
	if err != nil {
		return nil, err
	}
	if err := validateMigrationFiles(config.Migrations.Runner, migrations); err != nil {
		return nil, err
	}
	return &MigrateCheckResult{Dir: dir, Count: len(migrations)}, nil
}

func renderMigrationFiles(runner string, plan migrate.Plan) ([]migrate.File, error) {
	switch migrate.NormaliseRunner(runner) {
	case migrate.RunnerGoose:
		return (goose.Renderer{}).Render(plan)
	default:
		return nil, fmt.Errorf("unsupported migration runner %q; supported values: %s",
			strings.TrimSpace(strings.ToLower(runner)), migrate.RunnerGoose)
	}
}

func validateMigrationFiles(runner string, migrations []migrate.Migration) error {
	switch migrate.NormaliseRunner(runner) {
	case migrate.RunnerGoose:
		for _, migration := range migrations {
			if err := goose.Validate(migration.Content); err != nil {
				return fmt.Errorf("%s: %w", migration.Path, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported migration runner %q; supported values: %s",
			strings.TrimSpace(strings.ToLower(runner)), migrate.RunnerGoose)
	}
}

func snapshotPlanner(value string) (migrateplan.SnapshotPlanner, error) {
	switch dialect(value) {
	case "postgres", "postgresql", "pg":
		return pgplan.Planner{}, nil
	default:
		return nil, fmt.Errorf("unsupported migration planner dialect %q", value)
	}
}

func writeNewFile(path, content string) error {
	// #nosec G301 -- migration directory creation uses user-resolved project paths with standard permissions.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// #nosec G304,G302 -- migration output path is user-resolved; migration SQL uses standard project file permissions and never overwrites.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("migration file already exists: %s", path)
		}
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := file.WriteString(content); err != nil {
		return err
	}
	return nil
}
