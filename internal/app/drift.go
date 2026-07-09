package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
	"github.com/webdeveloperben/gosqlkit/kit"
)

type DriftCheckOptions struct {
	inspectPG              driftInspectFunc
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

type DriftCheckResult struct {
	DesiredSnapshotID  string            `json:"desiredSnapshotId"`
	DatabaseSnapshotID string            `json:"databaseSnapshotId"`
	Dialect            string            `json:"dialect"`
	Differences        []DriftDifference `json:"differences,omitempty"`
	Drift              bool              `json:"drift"`
}

type driftInspectFunc func(context.Context, string) (pgschema.Schema, error)

func DriftCheckWithConfig(config *Config, opts DriftCheckOptions) (*DriftCheckResult, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if err := requireDialectCapability(dialect(config.Dialect), kit.CapabilityDriftCheck); err != nil {
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

	desiredSnapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		return nil, err
	}
	var desiredDoc pgschema.Document
	if err := json.Unmarshal([]byte(desiredSnapshot), &desiredDoc); err != nil {
		return nil, fmt.Errorf("parse desired snapshot: %w", err)
	}
	desiredProjection := projectDriftDocument(desiredDoc)
	desiredID, err := driftSnapshotID(desiredProjection)
	if err != nil {
		return nil, fmt.Errorf("build desired drift snapshot: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var databaseSchema pgschema.Schema
	if opts.inspectPG != nil {
		databaseSchema, err = opts.inspectPG(ctx, databaseURL)
	} else {
		databaseSchema, err = inspectPostgresWithOptions(ctx, postgresConnectionOptions(databaseURL, auth))
	}
	if err != nil {
		return nil, err
	}
	databaseProjection := projectDriftSchema(databaseSchema)
	databaseID, err := driftSnapshotID(databaseProjection)
	if err != nil {
		return nil, fmt.Errorf("build database drift snapshot: %w", err)
	}

	result := &DriftCheckResult{
		Dialect:            "postgresql",
		DesiredSnapshotID:  desiredID,
		DatabaseSnapshotID: databaseID,
		Drift:              desiredID != databaseID,
		Differences:        driftDifferences(desiredProjection, databaseProjection),
	}
	if result.Drift {
		return result, fmt.Errorf("database schema drift detected: database snapshot %q does not match desired snapshot %q", databaseID, desiredID)
	}
	return result, nil
}

func inspectPostgres(ctx context.Context, rawURL string) (pgschema.Schema, error) {
	return inspectPostgresWithOptions(ctx, pgtooling.ConnectionOptions{URL: rawURL})
}

func inspectPostgresWithOptions(ctx context.Context, opts pgtooling.ConnectionOptions) (pgschema.Schema, error) {
	conn, err := pgtooling.OpenWithOptions(ctx, opts)
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
		Collations:        doc.Collations,
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
		Grants:            doc.Grants,
	})
}

func projectDriftSchema(schema pgschema.Schema) pgschema.Schema {
	tables := make([]pgschema.Table, 0, len(schema.Tables))
	for _, table := range schema.Tables {
		if isMigrationRunnerTable(table) {
			continue
		}
		tables = append(tables, projectDriftTable(table))
	}
	return pgschema.Schema{
		Namespaces:        clearNamespacePreviousNames(schema.Namespaces),
		Extensions:        clearExtensionPreviousNames(schema.Extensions),
		Roles:             projectDriftRoles(schema.Roles),
		Collations:        projectDriftCollations(schema.Collations),
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
		Grants:            projectDriftGrants(schema.Grants),
	}
}

func projectDriftCollations(items []pgschema.Collation) []pgschema.Collation {
	out := append([]pgschema.Collation(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].Version = ""
		if out[i].Provider == "libc" {
			out[i].Provider = ""
		}
		if out[i].Deterministic != nil && *out[i].Deterministic {
			out[i].Deterministic = nil
		}
	}
	return out
}

func isMigrationRunnerTable(table pgschema.Table) bool {
	schema := table.Schema
	if schema == "" {
		schema = "public"
	}
	if schema != "public" {
		return false
	}
	switch table.Name {
	case "goose_db_version", "schema_migrations":
		return true
	default:
		return false
	}
}

func projectDriftTable(table pgschema.Table) pgschema.Table {
	canonicaliseInlineConstraints(&table)
	columns := make([]ast.Column, 0, len(table.Columns))
	for _, column := range table.Columns {
		column.Type = normaliseDriftType(column.Type)
		column.Default = normaliseDriftExpression(column.Default)
		if column.Generated != nil {
			column.Generated.As = normaliseDriftExpression(column.Generated.As)
			if column.Generated.As == "" {
				column.Generated = nil
			}
		}
		if column.Identity != nil {
			column.Identity = &ast.Identity{Type: column.Identity.Type}
		}
		if column.References != nil {
			column.References.Table = normaliseDriftReference(column.References.Table)
			column.References.OnDelete = normaliseDriftAction(column.References.OnDelete)
			column.References.OnUpdate = normaliseDriftAction(column.References.OnUpdate)
		}
		if column.PrimaryKey {
			column.NotNull = false
		}
		column.PreviousName = ""
		columns = append(columns, column)
	}
	table.Columns = columns
	table.Tablespace = normaliseDriftTablespace(table.Tablespace)
	table.PrimaryKeys = clearPrimaryKeyPreviousNames(table.PrimaryKeys)
	table.UniqueConstraints = clearUniquePreviousNames(table.UniqueConstraints)
	table.ForeignKeys = clearForeignKeyPreviousNames(table.ForeignKeys)
	table.Checks = clearCheckPreviousNames(table.Checks)
	table.Indexes = projectDriftIndexes(table.Indexes)
	if table.Partitioning != nil {
		for i := range table.Partitioning.Keys {
			table.Partitioning.Keys[i].Expression = normaliseDriftExpression(table.Partitioning.Keys[i].Expression)
		}
	}
	if table.PartitionOf != nil {
		table.PartitionOf.Parent = normaliseDriftReference(table.PartitionOf.Parent)
		table.PartitionOf.Bound.From = normaliseDriftExpressions(table.PartitionOf.Bound.From)
		table.PartitionOf.Bound.To = normaliseDriftExpressions(table.PartitionOf.Bound.To)
		table.PartitionOf.Bound.Values = normaliseDriftExpressions(table.PartitionOf.Bound.Values)
	}
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
		for j := range out[i].Attributes {
			out[i].Attributes[j].Type = normaliseDriftType(out[i].Attributes[j].Type)
		}
	}
	return out
}

func clearDomainPreviousNames(items []pgschema.Domain) []pgschema.Domain {
	out := append([]pgschema.Domain(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].BaseType = normaliseDriftType(out[i].BaseType)
		out[i].Default = normaliseDriftExpression(out[i].Default)
		out[i].Check = normaliseDriftExpression(out[i].Check)
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
	function.ReturnType = normaliseDriftType(function.ReturnType)
	function.Body = normaliseSQLDefinition(function.Body)
	for i := range function.Arguments {
		function.Arguments[i].Type = normaliseDriftType(function.Arguments[i].Type)
		function.Arguments[i].Default = normaliseDriftExpression(function.Arguments[i].Default)
	}
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
	if function.Rows != nil && (*function.Rows == 0 || *function.Rows == 1000) {
		function.Rows = nil
	}
	return function
}

func projectDriftViews(items []pgschema.View) []pgschema.View {
	out := append([]pgschema.View(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].ColumnAliases = nil
		out[i].DependsOn = nil
		out[i].Query = ""
	}
	return out
}

func projectDriftMaterializedViews(items []pgschema.MaterializedView) []pgschema.MaterializedView {
	out := append([]pgschema.MaterializedView(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].Tablespace = normaliseDriftTablespace(out[i].Tablespace)
		out[i].ColumnAliases = nil
		out[i].DependsOn = nil
		out[i].NoData = false
		out[i].Query = ""
	}
	return out
}

func projectDriftTriggers(items []pgschema.Trigger) []pgschema.Trigger {
	out := append([]pgschema.Trigger(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].Target = normaliseDriftReference(out[i].Target)
		out[i].Function = normaliseDriftReference(out[i].Function)
		out[i].ReferencedTable = normaliseDriftReference(out[i].ReferencedTable)
		out[i].When = normaliseDriftExpression(out[i].When)
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
		out[i].Table = normaliseDriftReference(out[i].Table)
		out[i].Using = normaliseDriftExpression(out[i].Using)
		out[i].WithCheck = normaliseDriftExpression(out[i].WithCheck)
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

func projectDriftGrants(items []pgschema.Grant) []pgschema.Grant {
	out := append([]pgschema.Grant(nil), items...)
	for i := range out {
		switch out[i].Target.Type {
		case "table", "sequence", "type":
			if !out[i].Target.AllInSchema {
				out[i].Target.Name = normaliseDriftReference(out[i].Target.Name)
			}
		case "function":
			if !out[i].Target.AllInSchema {
				out[i].Target.Name = normaliseDriftFunctionReference(out[i].Target.Name)
			}
		}
		for j := range out[i].Privileges {
			out[i].Privileges[j].Name = strings.ToUpper(out[i].Privileges[j].Name)
		}
	}
	return out
}

func normaliseDriftFunctionReference(value string) string {
	name, signature, ok := strings.Cut(value, "(")
	if !ok {
		return normaliseDriftReference(value)
	}
	return normaliseDriftReference(name) + "(" + signature
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

func projectDriftIndexes(items []pgschema.Index) []pgschema.Index {
	out := append([]pgschema.Index(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].Concurrently = false
		out[i].Only = false
		out[i].Tablespace = normaliseDriftTablespace(out[i].Tablespace)
		if out[i].Method == "btree" {
			out[i].Method = ""
		}
		out[i].Where = normaliseDriftExpression(out[i].Where)
		for j := range out[i].Columns {
			out[i].Columns[j].Expression = normaliseDriftExpression(out[i].Columns[j].Expression)
			if out[i].Columns[j].OpClass == "text_ops" {
				out[i].Columns[j].OpClass = ""
			}
			if out[i].Columns[j].Order == "ASC" {
				out[i].Columns[j].Order = ""
			}
			if out[i].Columns[j].Order == "" && out[i].Columns[j].Nulls == "LAST" {
				out[i].Columns[j].Nulls = ""
			}
			if out[i].Columns[j].Order == "DESC" && out[i].Columns[j].Nulls == "FIRST" {
				out[i].Columns[j].Nulls = ""
			}
		}
	}
	return out
}

func normaliseDriftTablespace(value string) string {
	if value == "pg_default" {
		return ""
	}
	return value
}

func canonicaliseInlineConstraints(table *pgschema.Table) {
	primaryKeys := table.PrimaryKeys[:0]
	for _, item := range table.PrimaryKeys {
		if len(item.Columns) == 1 && item.Name == table.Name+"_pkey" {
			setDriftColumn(table, item.Columns[0], func(column *ast.Column) {
				column.PrimaryKey = true
			})
			continue
		}
		primaryKeys = append(primaryKeys, item)
	}
	table.PrimaryKeys = primaryKeys

	uniqueConstraints := table.UniqueConstraints[:0]
	for _, item := range table.UniqueConstraints {
		if len(item.Columns) == 1 && item.Name == table.Name+"_"+item.Columns[0]+"_key" && !item.Deferrable {
			setDriftColumn(table, item.Columns[0], func(column *ast.Column) {
				column.Unique = true
			})
			continue
		}
		uniqueConstraints = append(uniqueConstraints, item)
	}
	table.UniqueConstraints = uniqueConstraints

	foreignKeys := table.ForeignKeys[:0]
	for _, item := range table.ForeignKeys {
		if len(item.Columns) == 1 && len(item.ReferencedColumns) == 1 && item.Name == table.Name+"_"+item.Columns[0]+"_fkey" && !item.Deferrable {
			setDriftColumn(table, item.Columns[0], func(column *ast.Column) {
				column.References = &ast.ForeignKey{
					Table:    normaliseDriftReference(item.ReferencedTable),
					Column:   item.ReferencedColumns[0],
					OnDelete: normaliseDriftAction(item.OnDelete),
					OnUpdate: normaliseDriftAction(item.OnUpdate),
				}
			})
			continue
		}
		foreignKeys = append(foreignKeys, item)
	}
	table.ForeignKeys = foreignKeys
}

func setDriftColumn(table *pgschema.Table, name string, fn func(*ast.Column)) {
	for i := range table.Columns {
		if table.Columns[i].Name == name {
			fn(&table.Columns[i])
			return
		}
	}
}

func normaliseDriftType(value string) string {
	out := strings.TrimSpace(value)
	out = strings.ReplaceAll(out, ", ", ",")
	out = strings.ReplaceAll(out, " (", "(")
	return out
}

var driftCastPattern = regexp.MustCompile(`::(?:[a-zA-Z_][a-zA-Z0-9_]*\.)?[a-zA-Z_][a-zA-Z0-9_]*`)

var currentSettingParensPattern = regexp.MustCompile(`\((current_setting\([^)]*\))\)`)

func normaliseDriftExpression(value string) string {
	out := normaliseSQLDefinition(value)
	out = driftCastPattern.ReplaceAllString(out, "")
	out = strings.NewReplacer(
		"VALUE", "value",
		"OLD.", "old.",
		"NEW.", "new.",
		`": "`, `":"`,
	).Replace(out)
	out = currentSettingParensPattern.ReplaceAllString(out, "$1")
	return strings.TrimSpace(out)
}

func normaliseDriftExpressions(values []string) []string {
	out := append([]string(nil), values...)
	for i := range out {
		out[i] = normaliseDriftExpression(out[i])
	}
	return out
}

func normaliseDriftReference(value string) string {
	if value == "" || strings.Contains(value, ".") {
		return value
	}
	return "public." + value
}

func normaliseDriftAction(value string) string {
	return strings.ToLower(value)
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
		if out[i].Initially == "IMMEDIATE" {
			out[i].Initially = ""
		}
	}
	return out
}

func clearForeignKeyPreviousNames(items []ast.ForeignKeyConstraint) []ast.ForeignKeyConstraint {
	out := append([]ast.ForeignKeyConstraint(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].ReferencedTable = normaliseDriftReference(out[i].ReferencedTable)
		out[i].OnDelete = normaliseDriftAction(out[i].OnDelete)
		out[i].OnUpdate = normaliseDriftAction(out[i].OnUpdate)
		if out[i].Initially == "IMMEDIATE" {
			out[i].Initially = ""
		}
	}
	return out
}

func clearCheckPreviousNames(items []ast.Check) []ast.Check {
	out := append([]ast.Check(nil), items...)
	for i := range out {
		out[i].PreviousName = ""
		out[i].Expression = normaliseDriftExpression(out[i].Expression)
	}
	return out
}
