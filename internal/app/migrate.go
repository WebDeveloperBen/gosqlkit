package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	pgplan "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/golangmigrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
	"github.com/webdeveloperben/gosqlkit/kit"
)

type MigrateCreateOptions struct {
	CreatedAt                  time.Time
	inspectPG                  driftInspectFunc
	RenameDecider              RenameDecisionFunc
	FromURL                    string
	FromAWSProfile             string
	Dir                        string
	Runner                     string
	FromGCloudInstance         string
	Name                       string
	FromAWSRegion              string
	Interaction                InteractionMode
	FromURLEnv                 string
	FromTokenCommand           string
	NoDown                     bool
	AllowDestructive           bool
	Empty                      bool
	FromAzureCLIToken          bool
	FromAzureDefaultCredential bool
	FromAWSCLIToken            bool
	FromAWSIAMToken            bool
	FromGCloudADCToken         bool
	FromGCloudToken            bool
}

type InteractionMode string

const (
	InteractionAuto   InteractionMode = ""
	InteractionAlways InteractionMode = "always"
	InteractionNever  InteractionMode = "never"
)

type MigrateCreateResult struct {
	Dir   string   `json:"dir"`
	Files []string `json:"files"`
}

type RenameCandidate struct {
	Kind      string `json:"kind"`
	FromKey   string `json:"fromKey"`
	ToKey     string `json:"toKey"`
	FromName  string `json:"fromName"`
	ToName    string `json:"toName"`
	ParentKey string `json:"parentKey,omitempty"`
}

type RenameDecision struct {
	Candidate RenameCandidate `json:"candidate"`
	Accept    bool            `json:"accept"`
}

type RenameDecisionFunc func([]RenameCandidate) ([]RenameDecision, error)

type MigrateCheckOptions struct {
	sandboxReplay                 sandboxReplayFunc
	sandboxInspect                driftInspectFunc
	Dir                           string
	SandboxURL                    string
	SandboxURLEnv                 string
	SandboxTokenCommand           string
	SandboxAWSProfile             string
	SandboxAWSRegion              string
	SandboxGCloudInstance         string
	SandboxAzureCLIToken          bool
	SandboxAzureDefaultCredential bool
	SandboxAWSCLIToken            bool
	SandboxAWSIAMToken            bool
	SandboxGCloudADCToken         bool
	SandboxGCloudToken            bool
}

type MigrateCheckResult struct {
	Sandbox *SandboxReplayResult `json:"sandbox,omitempty"`
	Dir     string               `json:"dir"`
	Count   int                  `json:"count"`
}

type SandboxReplayResult struct {
	LastFile           string `json:"lastFile,omitempty"`
	ToSnapshotID       string `json:"toSnapshotId,omitempty"`
	DatabaseSnapshotID string `json:"databaseSnapshotId,omitempty"`
	Applied            int    `json:"applied"`
}

type sandboxReplayFunc func(context.Context, string, string, []migrate.Migration) (*SandboxReplayResult, error)

type MigrateApplyOptions struct {
	apply                  migrateApplyFunc
	Dir                    string
	Runner                 string
	URL                    string
	URLEnv                 string
	TokenCommand           string
	AWSProfile             string
	AWSRegion              string
	GCloudInstance         string
	AzureCLIToken          bool
	AzureDefaultCredential bool
	AWSCLIToken            bool
	AWSIAMToken            bool
	GCloudADCToken         bool
	GCloudToken            bool
}

type MigrateApplyResult struct {
	Dir      string `json:"dir"`
	LastFile string `json:"lastFile,omitempty"`
	Count    int    `json:"count"`
	Applied  int    `json:"applied"`
	Skipped  int    `json:"skipped"`
}

type migrateApplyFunc func(context.Context, string, string, []migrate.Migration) (*MigrateApplyResult, error)

type MigratePlanOptions struct {
	inspectPG                  driftInspectFunc
	Dir                        string
	Runner                     string
	Snapshot                   string
	FromURL                    string
	FromURLEnv                 string
	FromTokenCommand           string
	FromAWSProfile             string
	FromAWSRegion              string
	FromGCloudInstance         string
	FromAzureCLIToken          bool
	FromAzureDefaultCredential bool
	FromAWSCLIToken            bool
	FromAWSIAMToken            bool
	FromGCloudADCToken         bool
	FromGCloudToken            bool
}

type MigratePlanResult struct {
	Dialect        string               `json:"dialect"`
	FromSnapshotID string               `json:"fromSnapshotId,omitempty"`
	ToSnapshotID   string               `json:"toSnapshotId"`
	Changes        []migrate.Change     `json:"changes"`
	Statements     []migrate.Statement  `json:"upStatements"`
	Destructive    []migrateplan.Change `json:"destructiveChanges,omitempty"`
}

func MigrateCreateWithConfig(config *Config, opts MigrateCreateOptions) (*MigrateCreateResult, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if err := validateInteractionMode(opts.Interaction); err != nil {
		return nil, err
	}
	if opts.Name == "" {
		return nil, errors.New("migration name is required")
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

	downStatements := []migrate.Statement{}
	if opts.NoDown {
		downStatements = nil
	}

	plan := migrate.Plan{
		Name:           opts.Name,
		Dialect:        config.Dialect,
		CreatedAt:      opts.CreatedAt,
		Changes:        []migrate.Change{},
		UpStatements:   []migrate.Statement{},
		DownStatements: downStatements,
	}

	if !opts.Empty {
		planned, err := plannedMigration(config, dir, opts)
		if err != nil {
			return nil, err
		}
		plan = planned
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

func plannedMigration(config *Config, dir string, opts MigrateCreateOptions) (migrate.Plan, error) {
	if err := requireDialectCapability(dialect(config.Dialect), kit.CapabilityMigrationPlan); err != nil {
		return migrate.Plan{}, err
	}

	existing, err := migrate.ScanDir(dir)
	if err != nil {
		return migrate.Plan{}, err
	}
	if len(existing) > 0 && len(existing[len(existing)-1].Metadata.TargetSnapshot) == 0 {
		return migrate.Plan{}, errors.New("latest migration has no targetSnapshot metadata; cannot create diff migration")
	}

	sql, _, err := renderSQLWithConfig(config)
	if err != nil {
		return migrate.Plan{}, err
	}
	snapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		return migrate.Plan{}, err
	}
	snapshotID, err := snapshotIDFromJSON(snapshot)
	if err != nil {
		return migrate.Plan{}, err
	}
	if hasLiveMigrationSource(opts.FromURL, opts.FromURLEnv) {
		source, sourceID, err := migratePlanSourceSnapshot(config, MigratePlanOptions{
			inspectPG: opts.inspectPG, FromURL: opts.FromURL, FromURLEnv: opts.FromURLEnv, FromTokenCommand: opts.FromTokenCommand, FromAWSProfile: opts.FromAWSProfile, FromAWSRegion: opts.FromAWSRegion, FromGCloudInstance: opts.FromGCloudInstance, FromAzureCLIToken: opts.FromAzureCLIToken, FromAzureDefaultCredential: opts.FromAzureDefaultCredential, FromAWSCLIToken: opts.FromAWSCLIToken, FromAWSIAMToken: opts.FromAWSIAMToken, FromGCloudADCToken: opts.FromGCloudADCToken, FromGCloudToken: opts.FromGCloudToken,
		})
		if err != nil {
			return migrate.Plan{}, err
		}
		return diffMigrationPlan(config, opts, migrate.Migration{Metadata: migrate.Metadata{ToSnapshotID: sourceID, TargetSnapshot: source}}, snapshot, snapshotID)
	}

	if len(existing) == 0 {
		return baselineMigrationPlan(config, opts, sql, snapshot, snapshotID), nil
	}
	return diffMigrationPlan(config, opts, existing[len(existing)-1], snapshot, snapshotID)
}

func hasLiveMigrationSource(rawURL, envName string) bool {
	return strings.TrimSpace(rawURL) != "" || strings.TrimSpace(envName) != ""
}

func baselineMigrationPlan(config *Config, opts MigrateCreateOptions, sql, snapshot, snapshotID string) migrate.Plan {
	return migrate.Plan{
		Name:           opts.Name,
		Dialect:        config.Dialect,
		CreatedAt:      opts.CreatedAt,
		ToSnapshotID:   snapshotID,
		TargetSnapshot: snapshot,
		Changes: []migrate.Change{{
			Op:      "baseline",
			Object:  migrate.ObjectRef{Kind: "schema", Key: "schema"},
			Summary: "Create baseline schema from current gosqlkit definitions",
		}},
		UpStatements:   []migrate.Statement{{SQL: sql}},
		DownStatements: nil,
	}
}

func diffMigrationPlan(config *Config, opts MigrateCreateOptions, previous migrate.Migration, snapshot, snapshotID string) (migrate.Plan, error) {
	planner, err := snapshotPlanner(config.Dialect)
	if err != nil {
		return migrate.Plan{}, err
	}
	planned, err := planner.PlanSnapshotDiff(previous.Metadata.TargetSnapshot, []byte(snapshot))
	if err != nil {
		return migrate.Plan{}, err
	}
	if planned.HasDestructive() && opts.RenameDecider != nil {
		renamed, err := planWithRenameDecisions(planner, previous.Metadata.TargetSnapshot, snapshot, planned, opts.RenameDecider)
		if err != nil {
			return migrate.Plan{}, err
		}
		planned = renamed
	}
	if len(planned.Changes) == 0 {
		return migrate.Plan{}, errors.New("schema has no changes")
	}
	if planned.HasDestructive() && !opts.AllowDestructive {
		return migrate.Plan{}, destructiveGuardError(planned)
	}
	if len(planned.Statements) == 0 {
		return migrate.Plan{}, errors.New("migration plan has no executable SQL statements; manual migration authoring is required")
	}

	changes := make([]migrate.Change, 0, len(planned.Changes))
	upStatements := make([]migrate.Statement, 0, len(planned.Statements))
	for _, change := range planned.Changes {
		risks := make([]string, 0, len(change.Risks))
		for _, risk := range change.Risks {
			risks = append(risks, string(risk))
		}
		dependencies := make([]migrate.ObjectRef, 0, len(change.Dependencies))
		for _, dependency := range change.Dependencies {
			dependencies = append(dependencies, migrateObjectRef(dependency))
		}
		changes = append(changes, migrate.Change{
			Op:           string(change.Op),
			Object:       migrateObjectRef(change.Object),
			Summary:      change.Summary,
			Risks:        risks,
			Dependencies: dependencies,
			Reversible:   change.Reversible,
		})
		for _, statement := range change.Statements {
			upStatements = append(upStatements, migrate.Statement{SQL: statement.SQL})
		}
	}
	if len(upStatements) == 0 {
		for _, statement := range planned.Statements {
			upStatements = append(upStatements, migrate.Statement{SQL: statement})
		}
	}
	downStatements := reverseStatements(planned.Changes)
	if opts.NoDown {
		downStatements = nil
	}

	return migrate.Plan{
		Name:           opts.Name,
		Dialect:        config.Dialect,
		FromSnapshotID: previous.Metadata.ToSnapshotID,
		ToSnapshotID:   snapshotID,
		TargetSnapshot: snapshot,
		CreatedAt:      opts.CreatedAt,
		Changes:        changes,
		UpStatements:   upStatements,
		DownStatements: downStatements,
	}, nil
}

func migrateObjectRef(ref migrateplan.ObjectRef) migrate.ObjectRef {
	return migrate.ObjectRef{Kind: string(ref.Kind), Key: ref.Key}
}

func reverseStatements(changes []migrateplan.Change) []migrate.Statement {
	if len(changes) == 0 {
		return nil
	}
	out := make([]migrate.Statement, 0, len(changes))
	for i := len(changes) - 1; i >= 0; i-- {
		change := changes[i]
		if len(change.ReverseStatements) == 0 {
			out = append(out, migrate.Statement{SQL: noAutomaticDownStatement(change)})
			continue
		}
		for _, statement := range change.ReverseStatements {
			out = append(out, migrate.Statement{SQL: statement.SQL})
		}
	}
	return out
}

func noAutomaticDownStatement(change migrateplan.Change) string {
	target := strings.TrimSpace(strings.Join([]string{string(change.Object.Kind), change.Object.Key}, " "))
	if target == "" {
		target = strings.TrimSpace(change.Summary)
	}
	if target == "" {
		target = "unknown change"
	}
	return "-- no automatic down for: " + target
}

func MigrateCheckWithConfig(config *Config, opts MigrateCheckOptions) (*MigrateCheckResult, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	sandboxURL, err := resolveOptionalDatabaseURL(opts.SandboxURL, opts.SandboxURLEnv, "sandbox")
	if err != nil {
		return nil, err
	}
	sandboxAuth := databaseAuthOptions{
		TokenCommand:           opts.SandboxTokenCommand,
		AWSProfile:             opts.SandboxAWSProfile,
		AWSRegion:              opts.SandboxAWSRegion,
		GCloudInstance:         opts.SandboxGCloudInstance,
		AzureCLIToken:          opts.SandboxAzureCLIToken,
		AzureDefaultCredential: opts.SandboxAzureDefaultCredential,
		AWSCLIToken:            opts.SandboxAWSCLIToken,
		AWSIAMToken:            opts.SandboxAWSIAMToken,
		GCloudADCToken:         opts.SandboxGCloudADCToken,
		GCloudToken:            opts.SandboxGCloudToken,
	}
	if err := validateDatabaseAuthOptions(sandboxAuth); err != nil {
		return nil, err
	}
	if sandboxURL != "" {
		if err := requireDialectCapability(dialect(config.Dialect), kit.CapabilitySandboxReplay); err != nil {
			return nil, err
		}
	}

	dir := opts.Dir
	if dir == "" {
		dir = config.MigrationsDir()
	} else {
		dir = config.ResolvePath(dir)
	}

	migrations, err := migrate.ScanDir(dir)
	if err != nil {
		return nil, err
	}
	if err := validateMigrationFiles(config.Migrations.Runner, migrations); err != nil {
		return nil, err
	}
	result := &MigrateCheckResult{Dir: dir, Count: len(migrations)}
	if sandboxURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		sandbox, err := migrateCheckSandbox(ctx, config, sandboxURL, sandboxAuth, migrations, opts.sandboxReplay, opts.sandboxInspect)
		if err != nil {
			return nil, err
		}
		result.Sandbox = sandbox
	}
	return result, nil
}

func MigrateApplyWithConfig(config *Config, opts MigrateApplyOptions) (*MigrateApplyResult, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if err := requireDialectCapability(dialect(config.Dialect), kit.CapabilityMigrationApply); err != nil {
		return nil, err
	}
	databaseURL, err := resolveRequiredDatabaseURL(opts.URL, opts.URLEnv, "database")
	if err != nil {
		return nil, err
	}
	auth := databaseAuthOptions{
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
	if err := validateDatabaseAuthOptions(auth); err != nil {
		return nil, err
	}

	dir := opts.Dir
	if dir == "" {
		dir = config.MigrationsDir()
	} else {
		dir = config.ResolvePath(dir)
	}
	runner := opts.Runner
	if runner == "" {
		runner = config.Migrations.Runner
	}

	migrations, err := migrate.ScanDir(dir)
	if err != nil {
		return nil, err
	}
	if err := validateMigrationFiles(runner, migrations); err != nil {
		return nil, err
	}

	apply := opts.apply
	if apply == nil {
		apply = func(ctx context.Context, databaseURL, runner string, migrations []migrate.Migration) (*MigrateApplyResult, error) {
			return applyPostgresMigrations(ctx, postgresConnectionOptions(databaseURL, auth), runner, migrations)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	result, err := apply(ctx, databaseURL, runner, migrations)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &MigrateApplyResult{}
	}
	result.Dir = dir
	result.Count = len(migrations)
	return result, nil
}

func MigratePlanWithConfig(config *Config, opts MigratePlanOptions) (*MigratePlanResult, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if err := requireDialectCapability(dialect(config.Dialect), kit.CapabilityMigrationPlan); err != nil {
		return nil, err
	}

	snapshotData, _, err := renderSnapshotWithConfig(config, opts.Snapshot)
	if err != nil {
		return nil, err
	}
	snapshotID, err := snapshotIDFromJSON(snapshotData)
	if err != nil {
		return nil, err
	}

	fromSnapshot, fromSnapshotID, err := migratePlanSourceSnapshot(config, opts)
	if err != nil {
		return nil, err
	}

	planner, err := snapshotPlanner(config.Dialect)
	if err != nil {
		return nil, err
	}
	planned, err := planner.PlanSnapshotDiff(fromSnapshot, []byte(snapshotData))
	if err != nil {
		return nil, err
	}
	if len(planned.Changes) == 0 {
		return &MigratePlanResult{
			Dialect:        config.Dialect,
			FromSnapshotID: fromSnapshotID,
			ToSnapshotID:   snapshotID,
		}, nil
	}

	changes := make([]migrate.Change, 0, len(planned.Changes))
	statements := make([]migrate.Statement, 0, len(planned.Statements))
	for _, change := range planned.Changes {
		risks := make([]string, 0, len(change.Risks))
		for _, risk := range change.Risks {
			risks = append(risks, string(risk))
		}
		dependencies := make([]migrate.ObjectRef, 0, len(change.Dependencies))
		for _, dependency := range change.Dependencies {
			dependencies = append(dependencies, migrateObjectRef(dependency))
		}
		changes = append(changes, migrate.Change{
			Op:           string(change.Op),
			Object:       migrateObjectRef(change.Object),
			Summary:      change.Summary,
			Risks:        risks,
			Dependencies: dependencies,
			Reversible:   change.Reversible,
		})
		for _, statement := range change.Statements {
			statements = append(statements, migrate.Statement{SQL: statement.SQL})
		}
	}
	if len(statements) == 0 {
		for _, statement := range planned.Statements {
			statements = append(statements, migrate.Statement{SQL: statement})
		}
	}

	result := &MigratePlanResult{
		Dialect:        config.Dialect,
		FromSnapshotID: fromSnapshotID,
		ToSnapshotID:   snapshotID,
		Changes:        changes,
		Statements:     statements,
	}
	if destructive := planned.DestructiveChanges(); len(destructive) > 0 {
		destructiveOut := make([]migrateplan.Change, 0, len(destructive))
		destructiveOut = append(destructiveOut, destructive...)
		result.Destructive = destructiveOut
	}
	return result, nil
}

func migratePlanSourceSnapshot(config *Config, opts MigratePlanOptions) ([]byte, string, error) {
	fromURL, err := resolveOptionalDatabaseURL(opts.FromURL, opts.FromURLEnv, "source")
	if err != nil {
		return nil, "", err
	}
	if fromURL == "" {
		dir := opts.Dir
		if dir == "" {
			dir = config.MigrationsDir()
		} else {
			dir = config.ResolvePath(dir)
		}
		existing, err := migrate.ScanDir(dir)
		if err != nil {
			return nil, "", err
		}
		if len(existing) == 0 {
			return nil, "", errors.New("no existing migrations to diff against; use 'migrate create' to author an initial migration or pass --from-url")
		}
		latest := existing[len(existing)-1]
		if len(latest.Metadata.TargetSnapshot) == 0 {
			return nil, "", errors.New("latest migration has no targetSnapshot metadata; cannot compute plan")
		}
		return latest.Metadata.TargetSnapshot, latest.Metadata.ToSnapshotID, nil
	}
	auth := databaseAuthOptions{TokenCommand: opts.FromTokenCommand, AWSProfile: opts.FromAWSProfile, AWSRegion: opts.FromAWSRegion, GCloudInstance: opts.FromGCloudInstance, AzureCLIToken: opts.FromAzureCLIToken, AzureDefaultCredential: opts.FromAzureDefaultCredential, AWSCLIToken: opts.FromAWSCLIToken, AWSIAMToken: opts.FromAWSIAMToken, GCloudADCToken: opts.FromGCloudADCToken, GCloudToken: opts.FromGCloudToken}
	if err := validateDatabaseAuthOptions(auth); err != nil {
		return nil, "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var schema pgschema.Schema
	if opts.inspectPG != nil {
		schema, err = opts.inspectPG(ctx, fromURL)
	} else {
		schema, err = inspectPostgresWithOptions(ctx, postgresConnectionOptions(fromURL, auth))
	}
	if err != nil {
		return nil, "", err
	}
	raw, err := pgschema.JSON("postgresql", schema)
	if err != nil {
		return nil, "", err
	}
	snapshot, err := injectSnapshotIDs(string(raw), "")
	if err != nil {
		return nil, "", err
	}
	id, err := snapshotIDFromJSON(snapshot)
	if err != nil {
		return nil, "", err
	}
	return []byte(snapshot), id, nil
}

func renderMigrationFiles(runner string, plan migrate.Plan) ([]migrate.File, error) {
	switch migrate.NormaliseRunner(runner) {
	case migrate.RunnerGoose:
		return (goose.Renderer{}).Render(plan)
	case migrate.RunnerGolangMigrate:
		return (golangmigrate.Renderer{}).Render(plan)
	default:
		return nil, fmt.Errorf("unsupported migration runner %q; supported values: %s",
			strings.TrimSpace(strings.ToLower(runner)), migrate.SupportedRunners())
	}
}

func validateMigrationFiles(runner string, migrations []migrate.Migration) error {
	switch migrate.NormaliseRunner(runner) {
	case migrate.RunnerGoose:
		for _, migration := range migrations {
			if err := goose.Validate(migration.Content); err != nil {
				return fmt.Errorf("%s: %w", migration.Path, err)
			}
		}
		return nil
	case migrate.RunnerGolangMigrate:
		return golangmigrate.Validate(migrations)
	default:
		return fmt.Errorf("unsupported migration runner %q; supported values: %s",
			strings.TrimSpace(strings.ToLower(runner)), migrate.SupportedRunners())
	}
}

func migrateCheckSandbox(ctx context.Context, config *Config, sandboxURL string, auth databaseAuthOptions, migrations []migrate.Migration, replay sandboxReplayFunc, inspect driftInspectFunc) (*SandboxReplayResult, error) {
	if len(migrations) == 0 {
		return nil, errors.New("no migrations to replay")
	}
	if _, err := snapshotPlanner(config.Dialect); err != nil {
		return nil, err
	}
	latest := migrations[len(migrations)-1]
	if len(latest.Metadata.TargetSnapshot) == 0 {
		return nil, errors.New("latest migration has no targetSnapshot metadata; cannot compare replay target")
	}
	currentSnapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		return nil, err
	}
	currentSnapshotID, err := snapshotIDFromJSON(currentSnapshot)
	if err != nil {
		return nil, err
	}
	if latest.Metadata.ToSnapshotID != currentSnapshotID {
		return nil, fmt.Errorf("latest migration target snapshot %q does not match current schema snapshot %q",
			latest.Metadata.ToSnapshotID, currentSnapshotID)
	}

	if replay == nil {
		replay = func(ctx context.Context, sandboxURL, runner string, migrations []migrate.Migration) (*SandboxReplayResult, error) {
			return replayPostgresSandbox(ctx, postgresConnectionOptions(sandboxURL, auth), runner, migrations)
		}
	}
	result, err := replay(ctx, sandboxURL, config.Migrations.Runner, migrations)
	if err != nil {
		return nil, err
	}
	result.ToSnapshotID = latest.Metadata.ToSnapshotID
	if inspect == nil {
		inspect = inspectPostgres
	}
	var target pgschema.Document
	if err := json.Unmarshal(latest.Metadata.TargetSnapshot, &target); err != nil {
		return nil, fmt.Errorf("parse latest target snapshot: %w", err)
	}
	targetID, err := driftSnapshotID(projectDriftDocument(target))
	if err != nil {
		return nil, fmt.Errorf("build latest target drift snapshot: %w", err)
	}
	sandboxSchema, err := inspect(ctx, sandboxURL)
	if err != nil {
		return nil, err
	}
	databaseID, err := driftSnapshotID(projectDriftSchema(sandboxSchema))
	if err != nil {
		return nil, fmt.Errorf("build sandbox drift snapshot: %w", err)
	}
	result.DatabaseSnapshotID = databaseID
	if databaseID != targetID {
		return nil, fmt.Errorf("sandbox replay drift detected: database snapshot %q does not match migration target snapshot %q", databaseID, targetID)
	}
	return result, nil
}

func replayPostgresSandbox(ctx context.Context, opts pgtooling.ConnectionOptions, runner string, migrations []migrate.Migration) (*SandboxReplayResult, error) {
	conn, err := pgtooling.OpenWithOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	replayed, err := pgtooling.Replay(ctx, conn, pgtooling.ReplayOptions{
		Runner:     runner,
		Migrations: migrations,
	})
	if err != nil {
		return nil, err
	}
	return &SandboxReplayResult{
		Applied:  replayed.Applied,
		LastFile: replayed.LastFile,
	}, nil
}

func applyPostgresMigrations(ctx context.Context, opts pgtooling.ConnectionOptions, runner string, migrations []migrate.Migration) (*MigrateApplyResult, error) {
	conn, err := pgtooling.OpenWithOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	applied, err := pgtooling.Apply(ctx, conn, pgtooling.ApplyOptions{
		Runner:     runner,
		Migrations: migrations,
	})
	if err != nil {
		return nil, err
	}
	return &MigrateApplyResult{
		LastFile: applied.LastFile,
		Applied:  applied.Applied,
		Skipped:  applied.Skipped,
	}, nil
}

func snapshotPlanner(value string) (migrateplan.SnapshotPlanner, error) {
	switch dialect(value) {
	case "postgres", "postgresql", "pg":
		return pgplan.Planner{}, nil
	default:
		return nil, fmt.Errorf("unsupported migration planner dialect %q", value)
	}
}

func writeNewFile(path, content string) error {
	// #nosec G301 -- migration directory creation uses user-resolved project paths with standard permissions.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// #nosec G304,G302 -- migration output path is user-resolved; migration SQL uses standard project file permissions and never overwrites.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("migration file already exists: %s", path)
		}
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := file.WriteString(content); err != nil {
		return err
	}
	return nil
}

func destructiveGuardError(plan *migrateplan.Plan) error {
	destructive := plan.DestructiveChanges()
	lines := make([]string, 0, len(destructive)+3)
	lines = append(lines, fmt.Sprintf("migration plan contains %d destructive change(s); pass --allow-destructive to proceed:", len(destructive)))
	for _, change := range destructive {
		risks := make([]string, 0, len(change.Risks))
		for _, risk := range change.Risks {
			risks = append(risks, string(risk))
		}
		lines = append(lines, fmt.Sprintf("  - %s %s [%s]", change.Op, change.Object.Key, strings.Join(risks, ",")))
	}
	if hints := renameAnnotationHints(plan); len(hints) > 0 {
		lines = append(lines, "possible renames were detected; annotate the new schema object with previousName to accept rename intent:")
		for _, hint := range hints {
			lines = append(lines, "  - "+hint)
		}
	}
	return errors.New(strings.Join(lines, "\n"))
}

func validateInteractionMode(mode InteractionMode) error {
	switch mode {
	case InteractionAuto, InteractionAlways, InteractionNever:
		return nil
	default:
		return fmt.Errorf("unsupported interaction mode %q", mode)
	}
}

func renameAnnotationHints(plan *migrateplan.Plan) []string {
	if plan == nil {
		return nil
	}
	candidates := renameCandidatesFromPlan(plan)
	hints := make([]string, 0)
	for _, candidate := range candidates {
		hints = append(hints, previousNameHint(candidate))
	}
	return hints
}

func previousNameHint(candidate RenameCandidate) string {
	return fmt.Sprintf("%s %s: set previousName to %q", candidate.Kind, candidate.ToKey, objectLocalNameWithoutSignature(candidate.FromKey))
}

func objectLocalName(key string) string {
	if dot := strings.LastIndex(key, "."); dot >= 0 && dot < len(key)-1 {
		return key[dot+1:]
	}
	return key
}
