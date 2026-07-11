package tooling

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5"
)

type Executor interface {
	Exec(context.Context, string, ...any) error
}

type Queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

type PasswordProvider interface {
	Password(context.Context) (string, error)
}

type ConnectionOptions struct {
	PasswordProvider PasswordProvider
	URL              string
	CloudSQLInstance string
	CloudSQLIAMAuthN bool
}

type Conn struct {
	conn        *pgx.Conn
	closeDialer func() error
}

func Open(ctx context.Context, rawURL string) (*Conn, error) {
	return OpenWithOptions(ctx, ConnectionOptions{URL: rawURL})
}

func OpenWithOptions(ctx context.Context, opts ConnectionOptions) (*Conn, error) {
	rawURL := strings.TrimSpace(opts.URL)
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("postgres connection URL is required")
	}
	config, err := pgx.ParseConfig(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL URL %s: %w", RedactURL(rawURL), err)
	}
	if opts.PasswordProvider != nil && !opts.CloudSQLIAMAuthN {
		password, err := opts.PasswordProvider.Password(ctx)
		if err != nil {
			return nil, err
		}
		config.Password = password
	}
	var closeDialer func() error
	if instance := strings.TrimSpace(opts.CloudSQLInstance); instance != "" {
		dialerOptions := []cloudsqlconn.Option{cloudsqlconn.WithLazyRefresh()}
		if opts.CloudSQLIAMAuthN {
			dialerOptions = append(dialerOptions, cloudsqlconn.WithIAMAuthN())
		}
		dialer, err := cloudsqlconn.NewDialer(ctx, dialerOptions...)
		if err != nil {
			return nil, fmt.Errorf("create Cloud SQL connector dialer: %w", err)
		}
		closeDialer = dialer.Close
		config.TLSConfig = nil
		config.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.Dial(ctx, instance)
		}
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		if closeDialer != nil {
			_ = closeDialer()
		}
		return nil, fmt.Errorf("connect to PostgreSQL %s: %w", RedactURL(rawURL), err)
	}
	return &Conn{conn: conn, closeDialer: closeDialer}, nil
}

func (c *Conn) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := c.conn.Exec(ctx, sql, args...)
	return err
}

func (c *Conn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.conn.Query(ctx, sql, args...)
}

func (c *Conn) Close(ctx context.Context) error {
	err := c.conn.Close(ctx)
	if c.closeDialer != nil {
		if closeErr := c.closeDialer(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
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
