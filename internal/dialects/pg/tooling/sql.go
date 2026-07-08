package tooling

import "github.com/webdeveloperben/gosqlkit/internal/sqlsplit"

func SplitSQLStatements(sql string) []string {
	return sqlsplit.Statements(sql)
}
