package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

type InspectOptions struct {
	Stdout          io.Writer
	inspectPG       driftInspectFunc
	URL             string
	URLEnv          string
	TokenCommand    string
	Out             string
	DriftProjection bool
	JSON            bool
}

type InspectResult struct {
	Out             string `json:"out,omitempty"`
	SnapshotID      string `json:"snapshotId"`
	Dialect         string `json:"dialect"`
	DriftProjection bool   `json:"driftProjection"`
}

func InspectWithConfig(config *Config, opts InspectOptions) (*InspectResult, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}

	databaseURL, err := resolveRequiredDatabaseURL(opts.URL, opts.URLEnv, "database")
	if err != nil {
		return nil, err
	}

	switch dialect(config.Dialect) {
	case "postgres", "postgresql", "pg":
		return inspectPostgresSnapshot(config, opts, databaseURL)
	default:
		return nil, fmt.Errorf("unsupported inspect dialect %q", config.Dialect)
	}
}

func inspectPostgresSnapshot(config *Config, opts InspectOptions, databaseURL string) (*InspectResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var schema pgschema.Schema
	var err error
	if opts.inspectPG != nil {
		schema, err = opts.inspectPG(ctx, databaseURL)
	} else {
		schema, err = inspectPostgresWithOptions(ctx, postgresConnectionOptions(databaseURL, databaseAuthOptions{
			TokenCommand: opts.TokenCommand,
		}))
	}
	if err != nil {
		return nil, err
	}

	if opts.DriftProjection {
		schema = projectDriftSchema(schema)
	}

	raw, err := pgschema.JSON("postgresql", schema)
	if err != nil {
		return nil, err
	}
	snapshot, err := injectSnapshotIDs(string(raw), "")
	if err != nil {
		return nil, err
	}
	snapshotID, err := snapshotIDFromJSON(snapshot)
	if err != nil {
		return nil, err
	}

	out := config.ResolvePath(opts.Out)
	if out != "" {
		if err := writeOutput(out, snapshot); err != nil {
			return nil, err
		}
	}
	if opts.Out == "" || opts.JSON {
		if _, err := io.WriteString(opts.Stdout, snapshot); err != nil {
			return nil, err
		}
	}

	return &InspectResult{
		Out:             out,
		SnapshotID:      snapshotID,
		Dialect:         "postgresql",
		DriftProjection: opts.DriftProjection,
	}, nil
}
