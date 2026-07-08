package tooling

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
)

type ReplayOptions struct {
	Runner     string
	Migrations []migrate.Migration
}

type ReplayResult struct {
	LastFile string
	Applied  int
}

func Replay(ctx context.Context, exec Executor, opts ReplayOptions) (*ReplayResult, error) {
	if exec == nil {
		return nil, errors.New("postgres executor is required")
	}
	switch migrate.NormaliseRunner(opts.Runner) {
	case migrate.RunnerGoose:
		return replayGoose(ctx, exec, opts.Migrations)
	default:
		return nil, fmt.Errorf("unsupported migration runner %q; supported values: %s",
			strings.TrimSpace(strings.ToLower(opts.Runner)), migrate.RunnerGoose)
	}
}

func replayGoose(ctx context.Context, exec Executor, migrations []migrate.Migration) (*ReplayResult, error) {
	result := &ReplayResult{}
	for _, migration := range migrations {
		sql, err := goose.UpSQL(migration.Content)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", migration.Path, err)
		}
		if sql != "" {
			if err := exec.Exec(ctx, sql); err != nil {
				return nil, fmt.Errorf("%s: apply goose up: %w", migration.Path, err)
			}
		}
		result.Applied++
		result.LastFile = migration.Name
	}
	return result, nil
}
