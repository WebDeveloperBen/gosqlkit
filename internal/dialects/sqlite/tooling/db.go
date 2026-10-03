package tooling

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	_ "modernc.org/sqlite"
)

type Conn struct {
	db   *sql.DB
	conn *sql.Conn
}

func Open(ctx context.Context, rawURL string) (*Conn, error) {
	dsn, err := sqliteDSN(rawURL)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to SQLite database: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		_ = conn.Close()
		_ = db.Close()
		return nil, fmt.Errorf("enable SQLite foreign-key enforcement: %w", err)
	}
	var enabled int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil {
		_ = conn.Close()
		_ = db.Close()
		return nil, fmt.Errorf("verify SQLite foreign-key enforcement: %w", err)
	}
	if enabled != 1 {
		_ = conn.Close()
		_ = db.Close()
		return nil, errors.New("SQLite foreign-key enforcement could not be enabled")
	}
	return &Conn{db: db, conn: conn}, nil
}

func sqliteDSN(rawURL string) (string, error) {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return "", errors.New("SQLite database URL is required")
	}
	if !strings.HasPrefix(value, "sqlite:") {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse SQLite database URL: %w", err)
	}
	if parsed.Opaque != "" {
		return parsed.Opaque, nil
	}
	if parsed.Host != "" && parsed.Host != "localhost" {
		return "", errors.New("SQLite database URL must not specify a remote host")
	}
	path := parsed.Path
	if path == "/:memory:" {
		path = ":memory:"
	}
	if path == "" {
		return "", errors.New("SQLite database URL must include a file path or an in-memory database")
	}
	dsn := "file:" + path
	if parsed.RawQuery != "" {
		dsn += "?" + parsed.RawQuery
	}
	return dsn, nil
}

func (c *Conn) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return c.conn.ExecContext(ctx, query, args...)
}

func (c *Conn) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return c.conn.QueryContext(ctx, query, args...)
}

func (c *Conn) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return c.conn.QueryRowContext(ctx, query, args...)
}

func (c *Conn) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return c.conn.BeginTx(ctx, opts)
}

func (c *Conn) Close() error {
	if c == nil {
		return nil
	}
	var connErr error
	if c.conn != nil {
		connErr = c.conn.Close()
		c.conn = nil
	}
	var dbErr error
	if c.db != nil {
		dbErr = c.db.Close()
		c.db = nil
	}
	return errors.Join(connErr, dbErr)
}
