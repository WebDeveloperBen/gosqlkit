package tooling

import (
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/sqlsplit"
)

func SplitSQLStatements(sql string) []string {
	return sqlsplit.Statements(sql)
}

func migrationNonTransactional(migration migrate.Migration, statements []string) (bool, error) {
	hasNonTransactionalRisk := false
	hasOpaqueSQL := false
	for _, change := range migration.Metadata.Changes {
		hasOpaqueSQL = hasOpaqueSQL || change.Object.Kind == "raw_sql"
		for _, risk := range change.Risks {
			hasNonTransactionalRisk = hasNonTransactionalRisk || risk == "non-transactional"
		}
	}
	if hasNonTransactionalRisk {
		return true, nil
	}
	nonTransactional := false
	for _, statement := range statements {
		normalized := normalizePostgresStatement(statement)
		if isKnownNonTransactionalStatement(normalized) {
			nonTransactional = true
			continue
		}
		if hasOpaqueSQL && !isClassifiableTransactionalStatement(normalized) {
			return false, fmt.Errorf("cannot establish transaction safety for opaque PostgreSQL SQL statement %q", statement)
		}
	}
	return nonTransactional, nil
}

func normalizePostgresStatement(statement string) string {
	value := strings.TrimSpace(statement)
	for {
		switch {
		case strings.HasPrefix(value, "--"):
			if newline := strings.IndexByte(value, '\n'); newline >= 0 {
				value = strings.TrimSpace(value[newline+1:])
			} else {
				return ""
			}
		case strings.HasPrefix(value, "/*"):
			if end := strings.Index(value[2:], "*/"); end >= 0 {
				value = strings.TrimSpace(value[end+4:])
			} else {
				return ""
			}
		default:
			return strings.ToUpper(strings.Join(strings.Fields(value), " "))
		}
	}
}

func isKnownNonTransactionalStatement(statement string) bool {
	for _, prefix := range []string{
		"CREATE INDEX CONCURRENTLY ",
		"CREATE UNIQUE INDEX CONCURRENTLY ",
		"DROP INDEX CONCURRENTLY ",
		"REFRESH MATERIALIZED VIEW CONCURRENTLY ",
		"CREATE DATABASE ",
		"DROP DATABASE ",
		"CREATE TABLESPACE ",
		"DROP TABLESPACE ",
		"ALTER SYSTEM ",
		"VACUUM",
		"CREATE SUBSCRIPTION ",
		"PREPARE TRANSACTION ",
	} {
		if strings.HasPrefix(statement, prefix) {
			return true
		}
	}
	return strings.HasPrefix(statement, "REINDEX ") && strings.Contains(statement, " CONCURRENTLY ")
}

func isClassifiableTransactionalStatement(statement string) bool {
	if isKnownNonTransactionalStatement(statement) {
		return false
	}
	fields := strings.Fields(statement)
	if len(fields) == 0 {
		return true
	}
	switch fields[0] {
	case "ALTER", "COMMENT", "COPY", "CREATE", "DELETE", "DROP", "EXPLAIN",
		"GRANT", "INSERT", "REFRESH", "REINDEX", "RESET", "REVOKE", "SELECT",
		"SET", "SHOW", "TRUNCATE", "UPDATE", "WITH", "ANALYZE":
		return true
	default:
		return false
	}
}
