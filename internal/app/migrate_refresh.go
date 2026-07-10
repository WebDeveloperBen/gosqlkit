package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	pgplan "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

type MigrateRefreshOptions struct {
	CreatedAt    time.Time
	Name         string
	Dir          string
	Runner       string
	View         string
	Concurrently bool
}

func MigrateRefreshWithConfig(config *Config, opts MigrateRefreshOptions) (*MigrateCreateResult, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	view := strings.TrimSpace(opts.View)
	if view == "" {
		return nil, errors.New("materialized view name is required")
	}

	runner := opts.Runner
	if runner == "" {
		runner = config.Migrations.Runner
	}

	dir := opts.Dir
	if dir == "" {
		dir = config.MigrationsDir()
	} else {
		dir = config.ResolvePath(dir)
	}

	existing, err := migrate.ScanDir(dir)
	if err != nil {
		return nil, err
	}
	if len(existing) == 0 {
		return nil, fmt.Errorf("no migrations found in %s; run `gosqlkit migrate create` before refreshing a materialized view", dir)
	}
	latest := existing[len(existing)-1]
	if len(latest.Metadata.TargetSnapshot) == 0 {
		return nil, errors.New("latest migration has no targetSnapshot metadata; cannot author a refresh migration")
	}

	schema, name, err := resolveMaterializedView(config.Dialect, latest.Metadata.TargetSnapshot, view)
	if err != nil {
		return nil, err
	}

	sql, err := refreshMaterializedViewSQL(config.Dialect, schema, name, opts.Concurrently)
	if err != nil {
		return nil, err
	}

	key := qualifiedObjectName(schema, name)
	risks := []string{string(migrateplan.RiskLockHeavy)}
	summarySuffix := ""
	if opts.Concurrently {
		risks = []string{string(migrateplan.RiskRequiresDDLReview)}
		summarySuffix = " concurrently"
	}

	migrationName := opts.Name
	if migrationName == "" {
		migrationName = "refresh_" + strings.ReplaceAll(key, ".", "_")
		if opts.Concurrently {
			migrationName += "_concurrently"
		}
	}

	plan := migrate.Plan{
		Name:           migrationName,
		Dialect:        config.Dialect,
		CreatedAt:      opts.CreatedAt,
		FromSnapshotID: latest.Metadata.ToSnapshotID,
		ToSnapshotID:   latest.Metadata.ToSnapshotID,
		TargetSnapshot: string(latest.Metadata.TargetSnapshot),
		Changes: []migrate.Change{{
			Op:      "refresh",
			Object:  migrate.ObjectRef{Kind: string(migrateplan.ObjectKindMaterializedView), Key: key},
			Summary: "refresh materialized view " + key + summarySuffix,
			Risks:   risks,
		}},
		UpStatements:   []migrate.Statement{{SQL: sql}},
		DownStatements: nil,
	}

	files, err := renderMigrationFiles(runner, plan)
	if err != nil {
		return nil, err
	}

	written := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(dir, file.Name)
		if err := writeNewFile(path, file.Content); err != nil {
			return nil, err
		}
		written = append(written, path)
	}

	return &MigrateCreateResult{Dir: dir, Files: written}, nil
}

func resolveMaterializedView(dialectValue string, snapshot []byte, input string) (string, string, error) {
	switch dialect(dialectValue) {
	case "postgres", "postgresql", "pg":
		return resolvePostgresMaterializedView(snapshot, input)
	default:
		return "", "", fmt.Errorf("materialized view refresh is not supported for dialect %q", dialectValue)
	}
}

func resolvePostgresMaterializedView(snapshot []byte, input string) (string, string, error) {
	var doc pgschema.Document
	if err := json.Unmarshal(snapshot, &doc); err != nil {
		return "", "", fmt.Errorf("parse target snapshot: %w", err)
	}
	wantSchema, wantName := splitQualifiedName(input)
	for _, mv := range doc.MaterializedViews {
		if normaliseSchema(mv.Schema) == normaliseSchema(wantSchema) && mv.Name == wantName {
			return mv.Schema, mv.Name, nil
		}
	}
	available := make([]string, 0, len(doc.MaterializedViews))
	for _, mv := range doc.MaterializedViews {
		available = append(available, qualifiedObjectName(mv.Schema, mv.Name))
	}
	sort.Strings(available)
	if len(available) == 0 {
		return "", "", fmt.Errorf("materialized view %q not found; no materialized views are defined in the current schema", input)
	}
	return "", "", fmt.Errorf("materialized view %q not found; available materialized views: %s", input, strings.Join(available, ", "))
}

func refreshMaterializedViewSQL(dialectValue, schema, name string, concurrently bool) (string, error) {
	switch dialect(dialectValue) {
	case "postgres", "postgresql", "pg":
		return pgplan.RenderRefreshMaterializedView(schema, name, concurrently), nil
	default:
		return "", fmt.Errorf("materialized view refresh is not supported for dialect %q", dialectValue)
	}
}

func splitQualifiedName(input string) (string, string) {
	if i := strings.LastIndex(input, "."); i >= 0 {
		return input[:i], input[i+1:]
	}
	return "", input
}

func normaliseSchema(schema string) string {
	if schema == "" {
		return "public"
	}
	return schema
}

func qualifiedObjectName(schema, name string) string {
	if normaliseSchema(schema) == "public" {
		return name
	}
	return schema + "." + name
}
