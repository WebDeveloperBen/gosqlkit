package tooling

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/golangmigrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
	"github.com/webdeveloperben/gosqlkit/internal/sqlsplit"
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

// Replay executes every migration without consulting or updating runner state.
func Replay(ctx context.Context, conn *Conn, opts ReplayOptions) (*ReplayResult, error) {
	if conn == nil {
		return nil, errors.New("SQLite connection is required")
	}
	runner := migrate.NormaliseRunner(opts.Runner)
	if runner != migrate.RunnerGoose && runner != migrate.RunnerGolangMigrate {
		return nil, unsupportedRunner(opts.Runner)
	}
	result := &ReplayResult{}
	for _, migration := range opts.Migrations {
		statements, err := upStatements(runner, migration)
		if err != nil {
			return nil, err
		}
		if err := executeMigration(ctx, conn, statements, nil); err != nil {
			return nil, fmt.Errorf("%s: apply %s migration: %w", migrationLabel(migration), runner, err)
		}
		result.Applied++
		result.LastFile = migration.Name
	}
	return result, nil
}

// Apply executes migrations not yet recorded by the selected runner.
func Apply(ctx context.Context, conn *Conn, opts ApplyOptions) (*ApplyResult, error) {
	if conn == nil {
		return nil, errors.New("SQLite connection is required")
	}
	runner := migrate.NormaliseRunner(opts.Runner)
	if runner != migrate.RunnerGoose && runner != migrate.RunnerGolangMigrate {
		return nil, unsupportedRunner(opts.Runner)
	}
	if runner == migrate.RunnerGolangMigrate {
		if err := golangmigrate.Validate(opts.Migrations); err != nil {
			return nil, err
		}
		if err := ensureMigrateTable(ctx, conn); err != nil {
			return nil, err
		}
		return applyGolangMigrate(ctx, conn, opts.Migrations)
	}
	if err := ensureGooseTable(ctx, conn); err != nil {
		return nil, err
	}
	return applyGoose(ctx, conn, opts.Migrations)
}

func applyGoose(ctx context.Context, conn *Conn, migrations []migrate.Migration) (*ApplyResult, error) {
	applied, err := gooseVersions(ctx, conn)
	if err != nil {
		return nil, err
	}
	result := &ApplyResult{}
	for _, migration := range migrations {
		version, err := migrate.VersionID(migration.Name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", migrationLabel(migration), err)
		}
		if applied[version] {
			result.Skipped++
			continue
		}
		statements, err := upStatements(migrate.RunnerGoose, migration)
		if err != nil {
			return nil, err
		}
		record := func(exec sqlExecutor) error {
			_, err := exec.ExecContext(ctx, `INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, 1)`, version)
			return err
		}
		if err := executeMigration(ctx, conn, statements, record); err != nil {
			return nil, fmt.Errorf("%s: apply goose migration: %w", migrationLabel(migration), err)
		}
		applied[version] = true
		result.Applied++
		result.LastFile = migration.Name
	}
	return result, nil
}

func applyGolangMigrate(ctx context.Context, conn *Conn, migrations []migrate.Migration) (*ApplyResult, error) {
	version, dirty, err := currentMigrateVersion(ctx, conn)
	if err != nil {
		return nil, err
	}
	if dirty {
		return nil, fmt.Errorf("golang-migrate version %d is dirty; resolve it before applying more migrations", version)
	}
	result := &ApplyResult{}
	for _, migration := range migrations {
		next, err := migrate.VersionID(migration.Name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", migrationLabel(migration), err)
		}
		if next <= version {
			result.Skipped++
			continue
		}
		statements, err := upStatements(migrate.RunnerGolangMigrate, migration)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", migrationLabel(migration), err)
		}
		if err := setMigrateVersion(ctx, conn, next, true); err != nil {
			return nil, fmt.Errorf("%s: mark golang-migrate version dirty: %w", migrationLabel(migration), err)
		}
		clearDirty := func(exec sqlExecutor) error {
			_, err := exec.ExecContext(ctx, `UPDATE schema_migrations SET version = ?, dirty = 0`, next)
			return err
		}
		if err := executeMigration(ctx, conn, statements, clearDirty); err != nil {
			return nil, fmt.Errorf("%s: apply golang-migrate migration (version remains dirty): %w", migrationLabel(migration), err)
		}
		version = next
		result.Applied++
		result.LastFile = migration.Name
	}
	return result, nil
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type migrationExecutor struct {
	conn *Conn
}

func (e migrationExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return e.conn.Exec(ctx, query, args...)
}

// executeMigration wraps ordinary migrations atomically. PRAGMA foreign_keys is
// connection-scoped and cannot be changed in a transaction, so rebuild migrations
// that toggle it run statement-by-statement outside one. Enforcement is restored
// even when a statement fails; restoration errors are joined without losing the
// original migration failure.
func executeMigration(ctx context.Context, conn *Conn, statements []string, finalize func(sqlExecutor) error) (retErr error) {
	statements = nonemptyStatements(statements)
	usesFKPragma := false
	for _, statement := range statements {
		if isForeignKeyPragma(statement) {
			usesFKPragma = true
			break
		}
	}
	if !usesFKPragma {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}
		defer func() {
			if retErr != nil {
				if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
					retErr = errors.Join(retErr, fmt.Errorf("rollback migration: %w", rollbackErr))
				}
			}
		}()
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("execute SQL statement: %w", err)
			}
		}
		if finalize != nil {
			if err := finalize(tx); err != nil {
				return fmt.Errorf("record migration state: %w", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
		return nil
	}

	for _, statement := range statements {
		if _, err := conn.Exec(ctx, statement); err != nil {
			retErr = fmt.Errorf("execute SQL statement: %w", err)
			break
		}
	}
	if retErr == nil && finalize != nil {
		if err := finalize(migrationExecutor{conn}); err != nil {
			retErr = fmt.Errorf("record migration state: %w", err)
		}
	}
	if _, err := conn.Exec(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		retErr = errors.Join(retErr, fmt.Errorf("restore SQLite foreign-key enforcement: %w", err))
	} else {
		var restored int
		if err := conn.QueryRow(ctx, "PRAGMA foreign_keys").Scan(&restored); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("verify restored SQLite foreign-key enforcement: %w", err))
		} else if restored != 1 {
			retErr = errors.Join(retErr, errors.New("SQLite foreign-key enforcement remained disabled after migration"))
		}
	}
	return retErr
}

func upStatements(runner string, migration migrate.Migration) ([]string, error) {
	var statements []string
	var err error
	switch runner {
	case migrate.RunnerGoose:
		statements, err = goose.UpStatements(migration.Content)
	case migrate.RunnerGolangMigrate:
		if !strings.HasSuffix(migration.Name, ".up.sql") {
			return nil, fmt.Errorf("%s: golang-migrate migration filename must end in .up.sql", migrationLabel(migration))
		}
		statements = sqlsplit.StatementsWithTriggers(golangmigrate.UpSQL(migration.Content))
	}
	if err != nil {
		return nil, fmt.Errorf("%s: parse %s up migration: %w", migrationLabel(migration), runner, err)
	}
	return nonemptyStatements(statements), nil
}

func nonemptyStatements(statements []string) []string {
	out := make([]string, 0, len(statements))
	for _, statement := range statements {
		if strings.TrimSpace(statement) != "" {
			out = append(out, statement)
		}
	}
	return out
}

func isForeignKeyPragma(statement string) bool {
	upper := strings.ToUpper(strings.TrimSpace(statement))
	upper = strings.TrimSuffix(upper, ";")
	upper = strings.ReplaceAll(upper, " ", "")
	upper = strings.ReplaceAll(upper, "\t", "")
	upper = strings.ReplaceAll(upper, "\r", "")
	upper = strings.ReplaceAll(upper, "\n", "")
	return strings.HasPrefix(upper, "PRAGMAFOREIGN_KEYS=") || strings.HasPrefix(upper, "PRAGMAFOREIGN_KEYS(")
}

func ensureGooseTable(ctx context.Context, conn *Conn) error {
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create goose version table: %w", err)
	}
	return nil
}

func gooseVersions(ctx context.Context, conn *Conn) (map[int64]bool, error) {
	rows, err := conn.Query(ctx, `SELECT version_id, is_applied FROM goose_db_version ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("read goose version table: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	versions := make(map[int64]bool)
	for rows.Next() {
		var version int64
		var applied bool
		if err := rows.Scan(&version, &applied); err != nil {
			return nil, fmt.Errorf("scan goose version table: %w", err)
		}
		versions[version] = applied
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read goose version table: %w", err)
	}
	return versions, nil
}

func ensureMigrateTable(ctx context.Context, conn *Conn) error {
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER NOT NULL PRIMARY KEY,
		dirty INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("create golang-migrate version table: %w", err)
	}
	return nil
}

func currentMigrateVersion(ctx context.Context, conn *Conn) (int64, bool, error) {
	var version int64
	var dirty bool
	err := conn.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty)
	if errors.Is(err, sql.ErrNoRows) {
		return -1, false, nil
	}
	if err != nil {
		return -1, false, fmt.Errorf("read golang-migrate version table: %w", err)
	}
	return version, dirty, nil
}

func setMigrateVersion(ctx context.Context, conn *Conn, version int64, dirty bool) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin version-state transaction: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear golang-migrate version state: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, dirty) VALUES (?, ?)`, version, dirty); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("write golang-migrate version state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit golang-migrate version state: %w", err)
	}
	return nil
}

func unsupportedRunner(runner string) error {
	return fmt.Errorf("unsupported SQLite migration runner %q; supported values: %s", strings.TrimSpace(strings.ToLower(runner)), migrate.SupportedRunners())
}

func migrationLabel(migration migrate.Migration) string {
	if migration.Path != "" {
		return migration.Path
	}
	if migration.Name != "" {
		return migration.Name
	}
	return "migration"
}
