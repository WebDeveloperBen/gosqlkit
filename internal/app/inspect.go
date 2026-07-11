package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/kit"
)

type InspectOptions struct {
	Stdout                 io.Writer
	inspectPG              driftInspectFunc
	URL                    string
	URLEnv                 string
	Auth                   string
	TokenCommand           string
	Out                    string
	AWSProfile             string
	AWSRegion              string
	GCloudInstance         string
	DriftProjection        bool
	JSON                   bool
	AzureCLIToken          bool
	AzureDefaultCredential bool
	AWSCLIToken            bool
	AWSIAMToken            bool
	GCloudADCToken         bool
	GCloudToken            bool
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
	if err := requireDialectCapability(dialect(config.Dialect), kit.CapabilityInspectDatabase); err != nil {
		return nil, err
	}

	databaseURL, err := resolveRequiredDatabaseURL(opts.URL, opts.URLEnv, "database")
	if err != nil {
		return nil, err
	}
	auth := databaseAuthOptions{
		Auth:                   opts.Auth,
		TokenCommand:           opts.TokenCommand,
		AWSProfile:             opts.AWSProfile,
		AWSRegion:              opts.AWSRegion,
		GCloudInstance:         opts.GCloudInstance,
		AzureCLIToken:          opts.AzureCLIToken,
		AzureDefaultCredential: opts.AzureDefaultCredential,
		AWSCLIToken:            opts.AWSCLIToken,
		AWSIAMToken:            opts.AWSIAMToken,
		GCloudADCToken:         opts.GCloudADCToken,
		GCloudToken:            opts.GCloudToken,
	}
	auth, err = normaliseDatabaseAuthOptions(auth)
	if err != nil {
		return nil, err
	}

	switch dialect(config.Dialect) {
	case "postgres", "postgresql", "pg":
		return inspectPostgresSnapshot(config, opts, databaseURL, auth)
	default:
		return nil, fmt.Errorf("unsupported inspect dialect %q", config.Dialect)
	}
}

func inspectPostgresSnapshot(config *Config, opts InspectOptions, databaseURL string, auth databaseAuthOptions) (*InspectResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var schema pgschema.Schema
	var err error
	if opts.inspectPG != nil {
		schema, err = opts.inspectPG(ctx, databaseURL)
	} else {
		schema, err = inspectPostgresWithOptions(ctx, postgresConnectionOptions(databaseURL, auth))
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
