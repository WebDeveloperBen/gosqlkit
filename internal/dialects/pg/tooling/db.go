package tooling

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

type RefreshablePasswordProvider interface {
	PasswordProvider
	Refresh(context.Context) (string, error)
}

type ConnectionOptions struct {
	PasswordProvider PasswordProvider
	URL              string
	CloudSQLInstance string
	CloudSQLIAMAuthN bool
	CloudSQLIAMUser  bool
	RequireTLS       bool
	ConnectAttempts  int
	ConnectBackoff   time.Duration
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
	if opts.RequireTLS {
		if config.TLSConfig == nil || slices.ContainsFunc(config.Fallbacks, func(fallback *pgconn.FallbackConfig) bool {
			return fallback.TLSConfig == nil
		}) {
			return nil, errors.New("cloud SQL IAM database authentication requires TLS; set sslmode=require or use --cloud-sql-connector")
		}
	}
	if opts.CloudSQLIAMUser {
		config.User = cloudSQLIAMUsername(config.User)
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
	attempts := opts.ConnectAttempts
	if attempts < 1 {
		attempts = 1
	}
	backoff := opts.ConnectBackoff
	for attempt := 0; attempt < attempts; attempt++ {
		if opts.PasswordProvider != nil && !opts.CloudSQLIAMAuthN {
			var password string
			if attempt > 0 {
				if provider, ok := opts.PasswordProvider.(RefreshablePasswordProvider); ok {
					password, err = provider.Refresh(ctx)
				} else {
					password, err = opts.PasswordProvider.Password(ctx)
				}
			} else {
				password, err = opts.PasswordProvider.Password(ctx)
			}
			if err != nil {
				if closeDialer != nil {
					_ = closeDialer()
				}
				return nil, err
			}
			config.Password = password
		}
		conn, connectErr := pgx.ConnectConfig(ctx, config)
		if connectErr == nil {
			return &Conn{conn: conn, closeDialer: closeDialer}, nil
		}
		if attempt+1 < attempts && backoff > 0 {
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				connectErr = ctx.Err()
			case <-timer.C:
			}
		}
		if attempt+1 == attempts {
			if closeDialer != nil {
				_ = closeDialer()
			}
			return nil, fmt.Errorf("connect to PostgreSQL %s: %w", RedactURL(rawURL), connectErr)
		}
	}
	return nil, errors.New("postgres connection attempts exhausted")
}

func cloudSQLIAMUsername(username string) string {
	return strings.TrimSuffix(strings.TrimSpace(username), ".gserviceaccount.com")
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
