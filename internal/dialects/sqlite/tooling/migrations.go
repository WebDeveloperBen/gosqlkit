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

func executeMigration(ctx context.Context, conn *Conn, statements []string, finalize func(sqlExecutor) error) error {
	statements = nonemptyStatements(statements)
	body, usesFKGuard, err := splitForeignKeyGuard(statements)
	if err != nil {
		return err
	}
	if !usesFKGuard {
		return executeTransactionalMigration(ctx, conn, statements, finalize, false)
	}

	var migrationErr error
	if _, err := conn.Exec(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		migrationErr = fmt.Errorf("disable SQLite foreign-key enforcement: %w", err)
	} else {
		migrationErr = executeTransactionalMigration(ctx, conn, body, finalize, true)
	}
	restoreErr := restoreForeignKeys(context.WithoutCancel(ctx), conn)
	return errors.Join(migrationErr, restoreErr)
}

func executeTransactionalMigration(ctx context.Context, conn *Conn, statements []string, finalize func(sqlExecutor) error, checkFK bool) (retErr error) {
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
	if checkFK {
		if err := checkForeignKeys(ctx, tx); err != nil {
			return err
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

func checkForeignKeys(ctx context.Context, tx *sql.Tx) (retErr error) {
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("check SQLite foreign keys: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close SQLite foreign-key check rows: %w", err))
		}
	}()
	if rows.Next() {
		var table string
		var rowID any
		var parent string
		var foreignKey int
		if err := rows.Scan(&table, &rowID, &parent, &foreignKey); err != nil {
			return fmt.Errorf("read SQLite foreign-key check result: %w", err)
		}
		return fmt.Errorf("SQLite foreign-key check failed: table %q rowid %v references %q (foreign-key index %d)", table, rowID, parent, foreignKey)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read SQLite foreign-key check results: %w", err)
	}
	return nil
}

func restoreForeignKeys(ctx context.Context, conn *Conn) error {
	if _, err := conn.Exec(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("restore SQLite foreign-key enforcement: %w", err)
	}
	var restored int
	if err := conn.QueryRow(ctx, "PRAGMA foreign_keys").Scan(&restored); err != nil {
		return fmt.Errorf("verify restored SQLite foreign-key enforcement: %w", err)
	}
	if restored != 1 {
		return errors.New("SQLite foreign-key enforcement remained disabled after migration")
	}
	return nil
}

func splitForeignKeyGuard(statements []string) ([]string, bool, error) {
	type pragma struct {
		index   int
		enabled bool
	}
	var guards []pragma
	for index, statement := range statements {
		isPragma, enabled, err := parseForeignKeyPragma(statement)
		if err != nil {
			return nil, false, err
		}
		if isPragma {
			guards = append(guards, pragma{index: index, enabled: enabled})
		}
	}
	if len(guards) == 0 {
		return statements, false, nil
	}
	if len(guards) == 1 && guards[0].enabled {
		return statements, false, nil
	}
	if len(guards) != 2 || guards[0].index >= guards[1].index ||
		guards[0].enabled || !guards[1].enabled {
		return nil, false, fmt.Errorf("unsupported SQLite foreign-key pragma guard; expected PRAGMA foreign_keys=OFF before and PRAGMA foreign_keys=ON after migration statements (guards: %v, statements: %d)", guards, len(statements))
	}
	body := make([]string, 0, len(statements)-2)
	for index, statement := range statements {
		if index != guards[0].index && index != guards[1].index {
			body = append(body, statement)
		}
	}
	return body, true, nil
}

func parseForeignKeyPragma(statement string) (bool, bool, error) {
	normalized := strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(statement)), ""))
	normalized = strings.TrimSuffix(normalized, ";")
	const prefix = "PRAGMAFOREIGN_KEYS"
	if !strings.HasPrefix(normalized, prefix) {
		return false, false, nil
	}
	value := strings.TrimPrefix(normalized, prefix)
	switch {
	case strings.HasPrefix(value, "="):
		value = strings.TrimPrefix(value, "=")
	case strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")"):
		value = strings.TrimSuffix(strings.TrimPrefix(value, "("), ")")
	default:
		return true, false, fmt.Errorf("unsupported SQLite foreign-key pragma %q", statement)
	}
	switch value {
	case "OFF":
		return true, false, nil
	case "ON":
		return true, true, nil
	default:
		return true, false, fmt.Errorf("unsupported SQLite foreign-key pragma value in %q", statement)
	}
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
