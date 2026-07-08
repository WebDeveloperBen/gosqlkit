package tooling

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Executor interface {
	Exec(context.Context, string, ...any) error
}

type Queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

type Conn struct {
	conn *pgx.Conn
}

func Open(ctx context.Context, rawURL string) (*Conn, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("postgres connection URL is required")
	}
	conn, err := pgx.Connect(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL %s: %w", RedactURL(rawURL), err)
	}
	return &Conn{conn: conn}, nil
}

func (c *Conn) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := c.conn.Exec(ctx, sql, args...)
	return err
}

func (c *Conn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.conn.Query(ctx, sql, args...)
}

func (c *Conn) Close(ctx context.Context) error {
	return c.conn.Close(ctx)
}

func RedactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" {
		return "<redacted>"
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		if _, ok := parsed.User.Password(); ok {
			parsed.User = url.UserPassword(username, "xxxxx")
		}
	}
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
			query.Set(key, "xxxxx")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
