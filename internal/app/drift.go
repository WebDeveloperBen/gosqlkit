package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
)

type DriftCheckOptions struct {
	inspectPG driftInspectFunc
	URL       string
}

type DriftCheckResult struct {
	DesiredSnapshotID  string `json:"desiredSnapshotId"`
	DatabaseSnapshotID string `json:"databaseSnapshotId"`
	Dialect            string `json:"dialect"`
	Drift              bool   `json:"drift"`
}

type driftInspectFunc func(context.Context, string) (pgschema.Schema, error)

func DriftCheckWithConfig(config *Config, opts DriftCheckOptions) (*DriftCheckResult, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if opts.URL == "" {
		return nil, errors.New("database URL is required")
	}
	if _, err := snapshotPlanner(config.Dialect); err != nil {
		return nil, err
	}

	desiredSnapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		return nil, err
	}
	var desiredDoc pgschema.Document
	if err := json.Unmarshal([]byte(desiredSnapshot), &desiredDoc); err != nil {
		return nil, fmt.Errorf("parse desired snapshot: %w", err)
	}
	desiredID, err := driftSnapshotID(projectDriftDocument(desiredDoc))
	if err != nil {
		return nil, fmt.Errorf("build desired drift snapshot: %w", err)
	}

	inspect := opts.inspectPG
	if inspect == nil {
		inspect = inspectPostgres
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	databaseSchema, err := inspect(ctx, opts.URL)
	if err != nil {
		return nil, err
	}
	databaseID, err := driftSnapshotID(projectDriftSchema(databaseSchema))
	if err != nil {
		return nil, fmt.Errorf("build database drift snapshot: %w", err)
	}

	result := &DriftCheckResult{
		Dialect:            "postgresql",
		DesiredSnapshotID:  desiredID,
		DatabaseSnapshotID: databaseID,
		Drift:              desiredID != databaseID,
	}
	if result.Drift {
		return result, fmt.Errorf("database schema drift detected: database snapshot %q does not match desired snapshot %q", databaseID, desiredID)
	}
	return result, nil
}

func inspectPostgres(ctx context.Context, rawURL string) (pgschema.Schema, error) {
	conn, err := pgtooling.Open(ctx, rawURL)
	if err != nil {
		return pgschema.Schema{}, err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()
	return pgtooling.Introspect(ctx, conn)
}

func driftSnapshotID(schema pgschema.Schema) (string, error) {
	raw, err := pgschema.JSON("postgresql", schema)
	if err != nil {
		return "", err
	}
	snapshot, err := injectSnapshotIDs(string(raw), "")
	if err != nil {
		return "", err
	}
	return snapshotIDFromJSON(snapshot)
}

func projectDriftDocument(doc pgschema.Document) pgschema.Schema {
	return projectDriftSchema(pgschema.Schema{
		Namespaces:        doc.Namespaces,
		Extensions:        doc.Extensions,
		Roles:             doc.Roles,
		Enums:             doc.Enums,
		CompositeTypes:    doc.CompositeTypes,
		Domains:           doc.Domains,
		Sequences:         doc.Sequences,
		Functions:         doc.Functions,
		Tables:            doc.Tables,
		Views:             doc.Views,
		MaterializedViews: doc.MaterializedViews,
		Triggers:          doc.Triggers,
		Policies:          doc.Policies,
	})
}

func projectDriftSchema(schema pgschema.Schema) pgschema.Schema {
	tables := make([]ast.Table, 0, len(schema.Tables))
	for _, table := range schema.Tables {
		tables = append(tables, projectDriftTable(table))
	}
	return pgschema.Schema{
		Namespaces:        clearNamespacePreviousNames(schema.Namespaces),
		Extensions:        clearExtensionPreviousNames(schema.Extensions),
		Roles:             projectDriftRoles(schema.Roles),
		Enums:             clearEnumPreviousNames(schema.Enums),
		CompositeTypes:    clearCompositeTypePreviousNames(schema.CompositeTypes),
		Domains:           clearDomainPreviousNames(schema.Domains),
		Sequences:         projectDriftSequences(schema.Sequences),
		Functions:         projectDriftFunctions(schema.Functions),
		Tables:            tables,
		Views:             projectDriftViews(schema.Views),
		MaterializedViews: projectDriftMaterializedViews(schema.MaterializedViews),
		Triggers:          projectDriftTriggers(schema.Triggers),
		Policies:          projectDriftPolicies(schema.Policies),
	}
}

func projectDriftTable(table ast.Table) ast.Table {
	columns := make([]ast.Column, 0, len(table.Columns))
	for _, column := range table.Columns {
		if column.Identity != nil {
			column.Identity = &ast.Identity{Type: column.Identity.Type}
		}
		column.PreviousName = ""
		columns = append(columns, column)
	}
	table.Columns = columns
	table.PrimaryKeys = clearPrimaryKeyPreviousNames(table.PrimaryKeys)
	table.UniqueConstraints = clearUniquePreviousNames(table.UniqueConstraints)
	table.ForeignKeys = clearForeignKeyPreviousNames(table.ForeignKeys)
	table.Checks = clearCheckPreviousNames(table.Checks)
	table.Indexes = projectDriftIndexes(table.Indexes)
	table.Exclusions = nil
	table.PreviousName = ""
	return table
}

func clearNamespacePreviousNames(items []pgschema.Namespace) []pgschema.Namespace {
	out := append([]pgschema.Namespace(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
	}
	return out
}

func clearExtensionPreviousNames(items []pgschema.Extension) []pgschema.Extension {
	out := append([]pgschema.Extension(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
	}
	return out
}

func projectDriftRoles(items []pgschema.Role) []pgschema.Role {
	out := append([]pgschema.Role(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i] = normaliseRoleDefaults(out[i])
	}
	return out
}

func normaliseRoleDefaults(role pgschema.Role) pgschema.Role {
	if role.Login != nil && !*role.Login {
		role.Login = nil
	}
	if role.Superuser != nil && !*role.Superuser {
		role.Superuser = nil
	}
	if role.CreateDB != nil && !*role.CreateDB {
		role.CreateDB = nil
	}
	if role.CreateRole != nil && !*role.CreateRole {
		role.CreateRole = nil
	}
	if role.Inherit != nil && *role.Inherit {
		role.Inherit = nil
	}
	if role.Replication != nil && !*role.Replication {
		role.Replication = nil
	}
	if role.BypassRLS != nil && !*role.BypassRLS {
		role.BypassRLS = nil
	}
	if role.ConnectionLimit != nil && *role.ConnectionLimit == -1 {
		role.ConnectionLimit = nil
	}
	if role.ValidUntil == "infinity" {
		role.ValidUntil = ""
	}
	return role
}

func clearEnumPreviousNames(items []pgschema.Enum) []pgschema.Enum {
	out := append([]pgschema.Enum(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
	}
	return out
}

func clearCompositeTypePreviousNames(items []pgschema.CompositeType) []pgschema.CompositeType {
	out := append([]pgschema.CompositeType(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
	}
	return out
}

func clearDomainPreviousNames(items []pgschema.Domain) []pgschema.Domain {
	out := append([]pgschema.Domain(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
	}
	return out
}

func projectDriftFunctions(items []pgschema.Function) []pgschema.Function {
	out := append([]pgschema.Function(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i] = normaliseFunctionDefaults(out[i])
	}
	return out
}

func normaliseFunctionDefaults(function pgschema.Function) pgschema.Function {
	if function.Volatility == "VOLATILE" {
		function.Volatility = ""
	}
	if function.Parallel == "UNSAFE" {
		function.Parallel = ""
	}
	if function.Strict != nil && !*function.Strict {
		function.Strict = nil
	}
	if function.Cost != nil && *function.Cost == 100 {
		function.Cost = nil
	}
	if function.Rows != nil && *function.Rows == 1000 {
		function.Rows = nil
	}
	function.Body = normaliseSQLDefinition(function.Body)
	return function
}

func projectDriftViews(items []pgschema.View) []pgschema.View {
	out := append([]pgschema.View(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].ColumnAliases = nil
		out[i].DependsOn = nil
		out[i].Query = normaliseSQLDefinition(out[i].Query)
	}
	return out
}

func projectDriftMaterializedViews(items []pgschema.MaterializedView) []pgschema.MaterializedView {
	out := append([]pgschema.MaterializedView(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].ColumnAliases = nil
		out[i].DependsOn = nil
		out[i].NoData = false
		out[i].Query = normaliseSQLDefinition(out[i].Query)
	}
	return out
}

func projectDriftTriggers(items []pgschema.Trigger) []pgschema.Trigger {
	out := append([]pgschema.Trigger(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		if out[i].Level == "STATEMENT" {
			out[i].Level = ""
		}
	}
	return out
}

func projectDriftPolicies(items []pgschema.Policy) []pgschema.Policy {
	out := append([]pgschema.Policy(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		if out[i].Mode == "PERMISSIVE" {
			out[i].Mode = ""
		}
		if out[i].Command == "ALL" {
			out[i].Command = ""
		}
		if len(out[i].Roles) == 1 && out[i].Roles[0] == "public" {
			out[i].Roles = nil
		}
	}
	return out
}

func normaliseSQLDefinition(value string) string {
	return strings.TrimSuffix(strings.TrimSpace(value), ";")
}

func projectDriftSequences(items []pgschema.Sequence) []pgschema.Sequence {
	out := append([]pgschema.Sequence(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i] = normaliseSequenceDefaults(out[i])
	}
	return out
}

func normaliseSequenceDefaults(sequence pgschema.Sequence) pgschema.Sequence {
	if sequence.Increment == 1 {
		sequence.Increment = 0
	}
	if sequence.MinValue != nil && *sequence.MinValue == 1 {
		sequence.MinValue = nil
	}
	if sequence.MaxValue != nil && *sequence.MaxValue == 9223372036854775807 {
		sequence.MaxValue = nil
	}
	if sequence.StartWith != nil && *sequence.StartWith == 1 {
		sequence.StartWith = nil
	}
	if sequence.Cache != nil && *sequence.Cache == 1 {
		sequence.Cache = nil
	}
	return sequence
}

func projectDriftIndexes(items []ast.Index) []ast.Index {
	out := append([]ast.Index(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].Concurrently = false
		out[i].Only = false
	}
	return out
}

func clearPrimaryKeyPreviousNames(items []ast.PrimaryKey) []ast.PrimaryKey {
	out := append([]ast.PrimaryKey(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
	}
	return out
}

func clearUniquePreviousNames(items []ast.UniqueConstraint) []ast.UniqueConstraint {
	out := append([]ast.UniqueConstraint(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
	}
	return out
}

func clearForeignKeyPreviousNames(items []ast.ForeignKeyConstraint) []ast.ForeignKeyConstraint {
	out := append([]ast.ForeignKeyConstraint(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
	}
	return out
}

func clearCheckPreviousNames(items []ast.Check) []ast.Check {
	out := append([]ast.Check(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
	}
	return out
}
