package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	sqlitetooling "github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/tooling"
)

type sqliteInspectFunc func(context.Context, string) (sqliteschema.Schema, error)

type databaseTooling interface {
	Dialect() string
	Snapshot(context.Context) (string, error)
}

type postgresDatabaseTooling struct {
	inspect driftInspectFunc
	url     string
	auth    databaseAuthOptions
}

type sqliteDatabaseTooling struct {
	inspect sqliteInspectFunc
	url     string
}

func newDatabaseTooling(dialect, rawURL string, auth databaseAuthOptions, inspectPG driftInspectFunc, inspectSQLite sqliteInspectFunc) (databaseTooling, error) {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "postgresql", "pg":
		return postgresDatabaseTooling{url: rawURL, auth: auth, inspect: inspectPG}, nil
	case "sqlite", "sqlite3":
		return sqliteDatabaseTooling{url: rawURL, inspect: inspectSQLite}, nil
	default:
		return nil, fmt.Errorf("unsupported database tooling dialect %q", dialect)
	}
}

func (postgresDatabaseTooling) Dialect() string {
	return "postgresql"
}

func (t postgresDatabaseTooling) Snapshot(ctx context.Context) (string, error) {
	var schema pgschema.Schema
	var err error
	if t.inspect != nil {
		schema, err = t.inspect(ctx, t.url)
	} else {
		schema, err = inspectPostgresWithOptions(ctx, postgresConnectionOptions(t.url, t.auth))
	}
	if err != nil {
		return "", err
	}
	raw, err := pgschema.JSON("postgresql", schema)
	if err != nil {
		return "", err
	}
	return injectSnapshotIDs(string(raw), "")
}

func (sqliteDatabaseTooling) Dialect() string {
	return "sqlite"
}

func (t sqliteDatabaseTooling) Snapshot(ctx context.Context) (string, error) {
	var schema sqliteschema.Schema
	var err error
	if t.inspect != nil {
		schema, err = t.inspect(ctx, t.url)
	} else {
		schema, err = inspectSQLite(ctx, t.url)
	}
	if err != nil {
		return "", err
	}
	raw, err := sqliteschema.JSON("sqlite", schema)
	if err != nil {
		return "", err
	}
	return injectSnapshotIDs(string(raw), "")
}

func inspectSQLite(ctx context.Context, rawURL string) (sqliteschema.Schema, error) {
	conn, err := sqlitetooling.Open(ctx, rawURL)
	if err != nil {
		return sqliteschema.Schema{}, err
	}
	defer func() {
		_ = conn.Close()
	}()
	return sqlitetooling.Introspect(ctx, conn)
}
