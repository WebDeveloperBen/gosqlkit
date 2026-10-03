package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	sqlitetooling "github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/tooling"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
)

func applySQLiteMigrations(ctx context.Context, databaseURL, runner string, migrations []migrate.Migration) (*MigrateApplyResult, error) {
	conn, err := sqlitetooling.Open(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = conn.Close()
	}()
	applied, err := sqlitetooling.Apply(ctx, conn, sqlitetooling.ApplyOptions{Runner: runner, Migrations: migrations})
	if err != nil {
		return nil, err
	}
	return &MigrateApplyResult{LastFile: applied.LastFile, Applied: applied.Applied, Skipped: applied.Skipped}, nil
}

func replaySQLiteSandbox(ctx context.Context, databaseURL, runner string, migrations []migrate.Migration) (*SandboxReplayResult, error) {
	conn, err := sqlitetooling.Open(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = conn.Close()
	}()
	replayed, err := sqlitetooling.Replay(ctx, conn, sqlitetooling.ReplayOptions{Runner: runner, Migrations: migrations})
	if err != nil {
		return nil, err
	}
	return &SandboxReplayResult{Applied: replayed.Applied, LastFile: replayed.LastFile}, nil
}

func migrateCheckSQLiteSandbox(ctx context.Context, config *Config, _ string, migrations []migrate.Migration, replay sandboxReplayFunc, inspect sqliteInspectFunc) (result *SandboxReplayResult, retErr error) {
	if len(migrations) == 0 {
		return nil, errors.New("no migrations to replay")
	}
	if _, err := snapshotPlanner(config.Dialect); err != nil {
		return nil, err
	}
	latest := migrations[len(migrations)-1]
	if len(latest.Metadata.TargetSnapshot) == 0 {
		return nil, errors.New("latest migration has no targetSnapshot metadata; cannot compare replay target")
	}
	currentSnapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		return nil, err
	}
	currentSnapshotID, err := snapshotIDFromJSON(currentSnapshot)
	if err != nil {
		return nil, err
	}
	if latest.Metadata.ToSnapshotID != currentSnapshotID {
		return nil, fmt.Errorf("latest migration target snapshot %q does not match current schema snapshot %q", latest.Metadata.ToSnapshotID, currentSnapshotID)
	}

	databaseFile, err := os.CreateTemp("", "gosqlkit-sqlite-sandbox-*.db")
	if err != nil {
		return nil, fmt.Errorf("create isolated SQLite sandbox: %w", err)
	}
	databaseURL := databaseFile.Name()
	if err := databaseFile.Close(); err != nil {
		closeErr := fmt.Errorf("close isolated SQLite sandbox file: %w", err)
		return nil, errors.Join(closeErr, removeSQLiteSandboxFiles(databaseURL))
	}
	defer func() {
		retErr = errors.Join(retErr, removeSQLiteSandboxFiles(databaseURL))
	}()

	if replay == nil {
		replay = replaySQLiteSandbox
	}
	result, err = replay(ctx, databaseURL, config.Migrations.Runner, migrations)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("SQLite sandbox replay returned no result")
	}
	result.ToSnapshotID = latest.Metadata.ToSnapshotID

	if inspect == nil {
		inspect = inspectSQLite
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	schema, err := inspect(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	raw, err := sqliteschema.JSON("sqlite", projectSQLiteDriftSchema(schema))
	if err != nil {
		return nil, fmt.Errorf("build SQLite sandbox drift snapshot: %w", err)
	}
	databaseSnapshot, err := injectSnapshotIDs(string(raw), "")
	if err != nil {
		return nil, err
	}
	databaseID, err := snapshotIDFromJSON(databaseSnapshot)
	if err != nil {
		return nil, fmt.Errorf("build SQLite sandbox drift snapshot: %w", err)
	}
	targetSnapshot, err := projectDatabaseSnapshot(string(latest.Metadata.TargetSnapshot), "sqlite")
	if err != nil {
		return nil, fmt.Errorf("build latest SQLite target drift snapshot: %w", err)
	}
	targetID, err := snapshotIDFromJSON(targetSnapshot)
	if err != nil {
		return nil, fmt.Errorf("build latest SQLite target drift snapshot: %w", err)
	}
	result.DatabaseSnapshotID = databaseID
	if databaseID != targetID {
		differences, err := databaseSnapshotDifferences(targetSnapshot, databaseSnapshot, "sqlite")
		if err != nil {
			return nil, fmt.Errorf("compare SQLite sandbox replay snapshots: %w", err)
		}
		return nil, fmt.Errorf("SQLite sandbox replay drift detected: database snapshot %q does not match migration target snapshot %q; object differences: %+v", databaseID, targetID, differences)
	}
	return result, nil
}

func removeSQLiteSandboxFiles(databaseURL string) error {
	var cleanupErr error
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		path := databaseURL + suffix
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove isolated SQLite sandbox file %q: %w", path, err))
		}
	}
	return cleanupErr
}
