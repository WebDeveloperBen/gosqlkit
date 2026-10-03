package app

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/webdeveloperben/gosqlkit/kit"
)

type InspectOptions struct {
	Stdout                 io.Writer
	inspectPG              driftInspectFunc
	inspectSQLite          sqliteInspectFunc
	URL                    string
	URLEnv                 string
	Auth                   string
	TokenCommand           string
	Out                    string
	AWSProfile             string
	AWSRegion              string
	GCloudInstance         string
	CloudSQLConnector      bool
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
		CloudSQLConnector:      opts.CloudSQLConnector,
		AzureCLIToken:          opts.AzureCLIToken,
		AzureDefaultCredential: opts.AzureDefaultCredential,
		AWSCLIToken:            opts.AWSCLIToken,
		AWSIAMToken:            opts.AWSIAMToken,
		GCloudADCToken:         opts.GCloudADCToken,
		GCloudToken:            opts.GCloudToken,
	}
	if !isSQLiteDialect(config.Dialect) {
		auth, err = normaliseDatabaseAuthOptions(auth)
		if err != nil {
			return nil, err
		}
	}
	return inspectDatabaseSnapshot(config, opts, databaseURL, auth)
}

func inspectDatabaseSnapshot(config *Config, opts InspectOptions, databaseURL string, auth databaseAuthOptions) (*InspectResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tooling, err := newDatabaseTooling(config.Dialect, databaseURL, auth, opts.inspectPG, opts.inspectSQLite)
	if err != nil {
		return nil, err
	}
	snapshot, err := tooling.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	if opts.DriftProjection {
		snapshot, err = projectDatabaseSnapshot(snapshot, tooling.Dialect())
		if err != nil {
			return nil, err
		}
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
		Dialect:         tooling.Dialect(),
		DriftProjection: opts.DriftProjection,
	}, nil
}
