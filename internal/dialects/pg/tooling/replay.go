package tooling

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/golangmigrate"
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

type ApplyOptions struct {
	Runner     string
	Migrations []migrate.Migration
}

type ApplyResult struct {
	LastFile string
	Applied  int
	Skipped  int
}

type Database interface {
	Executor
	Queryer
	Begin(context.Context) (pgx.Tx, error)
}

func Replay(ctx context.Context, exec Executor, opts ReplayOptions) (*ReplayResult, error) {
	if exec == nil {
		return nil, errors.New("postgres executor is required")
	}
	switch migrate.NormaliseRunner(opts.Runner) {
	case migrate.RunnerGoose:
		return replayGoose(ctx, exec, opts.Migrations)
	case migrate.RunnerGolangMigrate:
		return replayGolangMigrate(ctx, exec, opts.Migrations)
	default:
		return nil, fmt.Errorf("unsupported migration runner %q; supported values: %s",
			strings.TrimSpace(strings.ToLower(opts.Runner)), migrate.SupportedRunners())
	}
}

func Apply(ctx context.Context, db Database, opts ApplyOptions) (*ApplyResult, error) {
	if db == nil {
		return nil, errors.New("postgres database is required")
	}
	switch migrate.NormaliseRunner(opts.Runner) {
	case migrate.RunnerGoose:
		return applyGoose(ctx, db, opts.Migrations)
	case migrate.RunnerGolangMigrate:
		return applyGolangMigrate(ctx, db, opts.Migrations)
	default:
		return nil, fmt.Errorf("unsupported migration runner %q; supported values: %s",
			strings.TrimSpace(strings.ToLower(opts.Runner)), migrate.SupportedRunners())
	}
}

func replayGoose(ctx context.Context, exec Executor, migrations []migrate.Migration) (*ReplayResult, error) {
	result := &ReplayResult{}
	for _, migration := range migrations {
		statements, err := goose.UpStatements(migration.Content)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", migration.Path, err)
		}
		for _, statement := range statements {
			if err := exec.Exec(ctx, statement); err != nil {
				return nil, fmt.Errorf("%s: apply goose up: %w", migration.Path, err)
			}
		}
		result.Applied++
		result.LastFile = migration.Name
	}
	return result, nil
}

func replayGolangMigrate(ctx context.Context, exec Executor, migrations []migrate.Migration) (*ReplayResult, error) {
	result := &ReplayResult{}
	for _, migration := range migrations {
		for _, statement := range SplitSQLStatements(golangmigrate.UpSQL(migration.Content)) {
			if err := exec.Exec(ctx, statement); err != nil {
				return nil, fmt.Errorf("%s: apply golang-migrate up: %w", migration.Path, err)
			}
		}
		result.Applied++
		result.LastFile = migration.Name
	}
	return result, nil
}

func applyGoose(ctx context.Context, db Database, migrations []migrate.Migration) (*ApplyResult, error) {
	if err := ensureGooseVersionTable(ctx, db); err != nil {
		return nil, err
	}
	applied, err := appliedGooseVersions(ctx, db)
	if err != nil {
		return nil, err
	}

	result := &ApplyResult{}
	for _, migration := range migrations {
		version, err := migrate.VersionID(migration.Name)
		if err != nil {
			return nil, err
		}
		if applied[version] {
			result.Skipped++
			continue
		}
		statements, err := goose.UpStatements(migration.Content)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", migration.Path, err)
		}
		nonTransactional, err := migrationNonTransactional(migration, statements)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", migration.Path, err)
		}
		if nonTransactional {
			if err := db.Exec(ctx, "INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, false);", version); err != nil {
				return nil, fmt.Errorf("%s: mark goose version unapplied: %w", migration.Path, err)
			}
			for _, statement := range statements {
				if err := db.Exec(ctx, statement); err != nil {
					return nil, fmt.Errorf("%s: apply goose up: %w", migration.Path, err)
				}
			}
			if err := db.Exec(ctx, "UPDATE goose_db_version SET is_applied = true WHERE id = (SELECT max(id) FROM goose_db_version WHERE version_id = $1);", version); err != nil {
				return nil, fmt.Errorf("%s: record goose version: %w", migration.Path, err)
			}
		} else if err := executeTransactionalMigration(ctx, db, statements, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, true);", version)
			return err
		}); err != nil {
			return nil, fmt.Errorf("%s: apply goose up transaction: %w", migration.Path, err)
		}
		applied[version] = true
		result.Applied++
		result.LastFile = migration.Name
	}
	return result, nil
}

func applyGolangMigrate(ctx context.Context, db Database, migrations []migrate.Migration) (*ApplyResult, error) {
	if err := ensureGolangMigrateVersionTable(ctx, db); err != nil {
		return nil, err
	}
	currentVersion, dirty, err := golangMigrateVersion(ctx, db)
	if err != nil {
		return nil, err
	}
	if dirty {
		return nil, fmt.Errorf("golang-migrate version %d is dirty", currentVersion)
	}

	result := &ApplyResult{}
	for _, migration := range migrations {
		version, err := migrate.VersionID(migration.Name)
		if err != nil {
			return nil, err
		}
		if version <= currentVersion {
			result.Skipped++
			continue
		}
		statements := SplitSQLStatements(golangmigrate.UpSQL(migration.Content))
		nonTransactional, err := migrationNonTransactional(migration, statements)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", migration.Path, err)
		}
		if err := setGolangMigrateVersion(ctx, db, version, true); err != nil {
			return nil, fmt.Errorf("%s: mark golang-migrate version dirty: %w", migration.Path, err)
		}
		if nonTransactional {
			for _, statement := range statements {
				if err := db.Exec(ctx, statement); err != nil {
					return nil, fmt.Errorf("%s: apply golang-migrate up: %w", migration.Path, err)
				}
			}
			if err := setGolangMigrateVersion(ctx, db, version, false); err != nil {
				return nil, fmt.Errorf("%s: record golang-migrate version: %w", migration.Path, err)
			}
		} else if err := executeTransactionalMigration(ctx, db, statements, func(tx pgx.Tx) error {
			return writeGolangMigrateVersion(ctx, tx, version, false)
		}); err != nil {
			return nil, fmt.Errorf("%s: apply golang-migrate up transaction (version remains dirty): %w", migration.Path, err)
		}
		currentVersion = version
		result.Applied++
		result.LastFile = migration.Name
	}
	return result, nil
}

func executeTransactionalMigration(ctx context.Context, db Database, statements []string, finalize func(pgx.Tx) error) (retErr error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer func() {
		if retErr != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				retErr = errors.Join(retErr, fmt.Errorf("rollback migration transaction: %w", rollbackErr))
			}
		}
	}()
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("execute migration statement: %w", err)
		}
	}
	if finalize != nil {
		if err := finalize(tx); err != nil {
			return fmt.Errorf("record migration state: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}
	return nil
}

func ensureGooseVersionTable(ctx context.Context, exec Executor) error {
	if err := exec.Exec(ctx, `CREATE TABLE IF NOT EXISTS goose_db_version (
    id integer GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    version_id bigint NOT NULL,
    is_applied boolean NOT NULL,
    tstamp timestamp NOT NULL DEFAULT now()
);`); err != nil {
		return fmt.Errorf("create goose version table: %w", err)
	}
	if err := exec.Exec(ctx, `INSERT INTO goose_db_version (version_id, is_applied)
SELECT 0, true
WHERE NOT EXISTS (SELECT 1 FROM goose_db_version WHERE version_id = 0);`); err != nil {
		return fmt.Errorf("initialise goose version table: %w", err)
	}
	return nil
}

func appliedGooseVersions(ctx context.Context, queryer Queryer) (map[int64]bool, error) {
	rows, err := queryer.Query(ctx, "SELECT version_id, is_applied FROM goose_db_version ORDER BY id;")
	if err != nil {
		return nil, fmt.Errorf("read goose version table: %w", err)
	}
	defer rows.Close()

	out := map[int64]bool{}
	for rows.Next() {
		var version int64
		var applied bool
		if err := rows.Scan(&version, &applied); err != nil {
			return nil, fmt.Errorf("scan goose version table: %w", err)
		}
		out[version] = applied
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read goose version table: %w", err)
	}
	return out, nil
}

func ensureGolangMigrateVersionTable(ctx context.Context, exec Executor) error {
	if err := exec.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
    version bigint NOT NULL PRIMARY KEY,
    dirty boolean NOT NULL
);`); err != nil {
		return fmt.Errorf("create golang-migrate version table: %w", err)
	}
	return nil
}

func golangMigrateVersion(ctx context.Context, queryer Queryer) (int64, bool, error) {
	rows, err := queryer.Query(ctx, "SELECT version, dirty FROM schema_migrations LIMIT 1;")
	if err != nil {
		return -1, false, fmt.Errorf("read golang-migrate version table: %w", err)
	}
	defer rows.Close()

	var version int64 = -1
	var dirty bool
	if rows.Next() {
		if err := rows.Scan(&version, &dirty); err != nil {
			return -1, false, fmt.Errorf("scan golang-migrate version table: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return -1, false, fmt.Errorf("read golang-migrate version table: %w", err)
	}
	return version, dirty, nil
}

func setGolangMigrateVersion(ctx context.Context, db Database, version int64, dirty bool) error {
	return executeTransactionalMigration(ctx, db, nil, func(tx pgx.Tx) error {
		return writeGolangMigrateVersion(ctx, tx, version, dirty)
	})
}

func writeGolangMigrateVersion(ctx context.Context, tx pgx.Tx, version int64, dirty bool) error {
	if _, err := tx.Exec(ctx, "TRUNCATE schema_migrations;"); err != nil {
		return fmt.Errorf("truncate golang-migrate version table: %w", err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, dirty) VALUES ($1, $2);", version, dirty); err != nil {
		return fmt.Errorf("write golang-migrate version table: %w", err)
	}
	return nil
}
