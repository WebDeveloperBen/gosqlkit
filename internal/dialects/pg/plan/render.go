package plan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

func renderRole(role pgschema.Role) string {
	var b strings.Builder
	b.WriteString("CREATE ROLE ")
	b.WriteString(role.Name)

	parts := []string{}
	parts = appendBoolRoleOption(parts, role.Superuser, "SUPERUSER", "NOSUPERUSER")
	parts = appendBoolRoleOption(parts, role.CreateDB, "CREATEDB", "NOCREATEDB")
	parts = appendBoolRoleOption(parts, role.CreateRole, "CREATEROLE", "NOCREATEROLE")
	parts = appendBoolRoleOption(parts, role.Inherit, "INHERIT", "NOINHERIT")
	parts = appendBoolRoleOption(parts, role.Login, "LOGIN", "NOLOGIN")
	parts = appendBoolRoleOption(parts, role.Replication, "REPLICATION", "NOREPLICATION")
	parts = appendBoolRoleOption(parts, role.BypassRLS, "BYPASSRLS", "NOBYPASSRLS")
	if role.ConnectionLimit != nil {
		parts = append(parts, fmt.Sprintf("CONNECTION LIMIT %d", *role.ConnectionLimit))
	}
	if role.ValidUntil != "" {
		parts = append(parts, "VALID UNTIL "+quoteSQL(role.ValidUntil))
	}
	if len(role.MemberOf) > 0 {
		parts = append(parts, "IN ROLE "+strings.Join(sortedStrings(role.MemberOf), ", "))
	}
	if len(role.AdminOf) > 0 {
		parts = append(parts, "ADMIN "+strings.Join(sortedStrings(role.AdminOf), ", "))
	}
	if len(parts) > 0 {
		b.WriteString(" WITH ")
		b.WriteString(strings.Join(parts, " "))
	}
	b.WriteString(";")
	return b.String()
}

func appendBoolRoleOption(parts []string, value *bool, on, off string) []string {
	if value == nil {
		return parts
	}
	if *value {
		return append(parts, on)
	}
	return append(parts, off)
}

func renderGrant(grant pgschema.Grant) string {
	var b strings.Builder
	b.WriteString("GRANT ")
	b.WriteString(renderGrantPrivileges(grant.Privileges))
	b.WriteString(" ON ")
	b.WriteString(renderGrantTarget(grant.Target))
	b.WriteString(" TO ")
	b.WriteString(strings.Join(sortedStrings(grant.Grantees), ", "))
	if grant.GrantOption {
		b.WriteString(" WITH GRANT OPTION")
	}
	b.WriteString(";")
	return b.String()
}

func renderRevokeGrant(grant pgschema.Grant) string {
	var b strings.Builder
	b.WriteString("REVOKE ")
	b.WriteString(renderGrantPrivileges(grant.Privileges))
	b.WriteString(" ON ")
	b.WriteString(renderGrantTarget(grant.Target))
	b.WriteString(" FROM ")
	b.WriteString(strings.Join(sortedStrings(grant.Grantees), ", "))
	b.WriteString(";")
	return b.String()
}

func renderGrantPrivileges(privileges []pgschema.GrantPrivilege) string {
	items := append([]pgschema.GrantPrivilege(nil), privileges...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return strings.Join(sortedStrings(items[i].Columns), "\x00") < strings.Join(sortedStrings(items[j].Columns), "\x00")
	})
	parts := make([]string, 0, len(items))
	for _, privilege := range items {
		part := strings.ToUpper(privilege.Name)
		if len(privilege.Columns) > 0 {
			part += " (" + strings.Join(sortedStrings(privilege.Columns), ", ") + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func renderGrantTarget(target pgschema.GrantTarget) string {
	if target.AllInSchema {
		switch target.Type {
		case "table":
			return "ALL TABLES IN SCHEMA " + target.Schema
		case "sequence":
			return "ALL SEQUENCES IN SCHEMA " + target.Schema
		case "function":
			return "ALL FUNCTIONS IN SCHEMA " + target.Schema
		}
	}
	switch target.Type {
	case "table":
		return "TABLE " + renderReferencedTable(target.Name)
	case "sequence":
		return "SEQUENCE " + renderReferencedTable(target.Name)
	case "schema":
		return "SCHEMA " + target.Name
	case "function":
		return "FUNCTION " + target.Name
	case "type":
		return "TYPE " + renderReferencedTable(target.Name)
	case "database":
		return "DATABASE " + target.Name
	default:
		return strings.ToUpper(target.Type) + " " + target.Name
	}
}

func renderSequence(sequence pgschema.Sequence, includeOwnedBy bool) string {
	var b strings.Builder
	b.WriteString("CREATE SEQUENCE ")
	b.WriteString(renderQualified(sequence.Schema, sequence.Name))
	if sequence.Increment != 0 {
		fmt.Fprintf(&b, " INCREMENT %d", sequence.Increment)
	}
	if sequence.MinValue != nil {
		fmt.Fprintf(&b, " MINVALUE %d", *sequence.MinValue)
	}
	if sequence.MaxValue != nil {
		fmt.Fprintf(&b, " MAXVALUE %d", *sequence.MaxValue)
	}
	if sequence.StartWith != nil {
		fmt.Fprintf(&b, " START %d", *sequence.StartWith)
	}
	if sequence.Cache != nil {
		fmt.Fprintf(&b, " CACHE %d", *sequence.Cache)
	}
	if sequence.Cycle {
		b.WriteString(" CYCLE")
	}
	if includeOwnedBy && sequence.OwnedBy != "" {
		b.WriteString(" OWNED BY ")
		b.WriteString(sequence.OwnedBy)
	}
	b.WriteString(";")
	return b.String()
}

func renderCompositeType(compositeType pgschema.CompositeType) (string, error) {
	if len(compositeType.Attributes) == 0 {
		return "", fmt.Errorf("composite type %q must have at least one attribute", renderQualified(compositeType.Schema, compositeType.Name))
	}

	var b strings.Builder
	b.WriteString("CREATE TYPE ")
	b.WriteString(renderQualified(compositeType.Schema, compositeType.Name))
	b.WriteString(" AS (\n")
	lines := make([]string, 0, len(compositeType.Attributes))
	for _, attribute := range compositeType.Attributes {
		if strings.TrimSpace(attribute.Type) == "" {
			return "", fmt.Errorf("composite type %q attribute %q must have a type", compositeType.Name, attribute.Name)
		}
		lines = append(lines, "    "+attribute.Name+" "+attribute.Type)
	}
	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n);")
	return b.String(), nil
}

func renderDomain(domain pgschema.Domain) (string, error) {
	if strings.TrimSpace(domain.BaseType) == "" {
		return "", fmt.Errorf("domain %q must have a base type", renderQualified(domain.Schema, domain.Name))
	}

	var b strings.Builder
	b.WriteString("CREATE DOMAIN ")
	b.WriteString(renderQualified(domain.Schema, domain.Name))
	b.WriteString(" AS ")
	b.WriteString(domain.BaseType)
	if domain.Default != "" {
		b.WriteString(" DEFAULT ")
		b.WriteString(domain.Default)
	}
	if domain.NotNull {
		b.WriteString(" NOT NULL")
	}
	if domain.Check != "" {
		b.WriteString(" CHECK (")
		b.WriteString(domain.Check)
		b.WriteString(")")
	}
	b.WriteString(";")
	return b.String(), nil
}

func renderFunction(function pgschema.Function) (string, error) {
	if strings.TrimSpace(function.Language) == "" {
		return "", fmt.Errorf("function %q must have a language", functionKey(function))
	}
	if strings.TrimSpace(function.ReturnType) == "" {
		return "", fmt.Errorf("function %q must have a return type", functionKey(function))
	}
	if strings.TrimSpace(function.Body) == "" {
		return "", fmt.Errorf("function %q must have a body", functionKey(function))
	}

	var b strings.Builder
	b.WriteString("CREATE FUNCTION ")
	b.WriteString(renderQualified(function.Schema, function.Name))
	b.WriteString("(")
	args := make([]string, 0, len(function.Arguments))
	for _, arg := range function.Arguments {
		args = append(args, renderFunctionArgument(arg))
	}
	b.WriteString(strings.Join(args, ", "))
	b.WriteString(")\nRETURNS ")
	b.WriteString(function.ReturnType)
	b.WriteString("\nLANGUAGE ")
	b.WriteString(function.Language)
	if function.Volatility != "" {
		b.WriteString("\n")
		b.WriteString(strings.ToUpper(function.Volatility))
	}
	if function.Strict != nil {
		if *function.Strict {
			b.WriteString("\nSTRICT")
		} else {
			b.WriteString("\nCALLED ON NULL INPUT")
		}
	}
	if function.SecurityDefiner {
		b.WriteString("\nSECURITY DEFINER")
	}
	if function.Parallel != "" {
		b.WriteString("\nPARALLEL ")
		b.WriteString(strings.ToUpper(function.Parallel))
	}
	if function.Cost != nil {
		fmt.Fprintf(&b, "\nCOST %g", *function.Cost)
	}
	if function.Rows != nil {
		fmt.Fprintf(&b, "\nROWS %d", *function.Rows)
	}
	for _, setting := range renderFunctionConfiguration(function.Configuration) {
		b.WriteString("\nSET ")
		b.WriteString(setting)
	}
	b.WriteString("\nAS $$\n")
	b.WriteString(function.Body)
	b.WriteString("\n$$;")
	return b.String(), nil
}

func renderFunctionArgument(arg pgschema.FunctionArgument) string {
	parts := []string{}
	if arg.Mode != "" {
		parts = append(parts, strings.ToUpper(arg.Mode))
	}
	if arg.Name != "" {
		parts = append(parts, arg.Name)
	}
	parts = append(parts, arg.Type)
	if arg.Default != "" {
		parts = append(parts, "DEFAULT", arg.Default)
	}
	return strings.Join(parts, " ")
}

func renderFunctionConfiguration(config map[string]string) []string {
	if len(config) == 0 {
		return nil
	}
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	settings := make([]string, 0, len(keys))
	for _, key := range keys {
		settings = append(settings, key+" = "+config[key])
	}
	return settings
}

func renderTrigger(trigger pgschema.Trigger) string {
	var b strings.Builder
	b.WriteString("CREATE ")
	if trigger.Constraint {
		b.WriteString("CONSTRAINT ")
	}
	b.WriteString("TRIGGER ")
	b.WriteString(trigger.Name)
	b.WriteString("\n")
	b.WriteString(strings.ToUpper(trigger.Timing))
	b.WriteString(" ")
	b.WriteString(renderTriggerEvents(trigger))
	b.WriteString(" ON ")
	b.WriteString(renderReferencedTable(trigger.Target))
	if trigger.ReferencedTable != "" {
		b.WriteString("\nFROM ")
		b.WriteString(renderReferencedTable(trigger.ReferencedTable))
	}
	if trigger.Deferrable {
		b.WriteString(renderDeferrable(true, trigger.Initially))
	}
	level := trigger.Level
	if level == "" {
		if trigger.Constraint || strings.EqualFold(trigger.Timing, "INSTEAD OF") {
			level = "ROW"
		} else {
			level = "STATEMENT"
		}
	}
	b.WriteString("\nFOR EACH ")
	b.WriteString(strings.ToUpper(level))
	if trigger.When != "" {
		b.WriteString("\nWHEN (")
		b.WriteString(trigger.When)
		b.WriteString(")")
	}
	b.WriteString("\nEXECUTE FUNCTION ")
	b.WriteString(renderReferencedTable(trigger.Function))
	b.WriteString("(")
	args := make([]string, 0, len(trigger.Arguments))
	for _, arg := range trigger.Arguments {
		args = append(args, quoteSQL(arg))
	}
	b.WriteString(strings.Join(args, ", "))
	b.WriteString(");")
	return b.String()
}

func renderTriggerEvents(trigger pgschema.Trigger) string {
	events := normalisedTriggerEvents(trigger.Events)
	parts := make([]string, 0, len(events))
	for _, event := range events {
		if event == "UPDATE" && len(trigger.Columns) > 0 {
			parts = append(parts, "UPDATE OF "+strings.Join(trigger.Columns, ", "))
			continue
		}
		parts = append(parts, event)
	}
	return strings.Join(parts, " OR ")
}

func normalisedTriggerEvents(events []string) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, strings.ToUpper(event))
	}
	sort.Strings(out)
	return out
}

func renderView(view pgschema.View) string {
	var b strings.Builder
	b.WriteString("CREATE ")
	if view.SecurityBarrier {
		b.WriteString("SECURITY BARRIER ")
	}
	if view.SecurityInvoker {
		b.WriteString("SECURITY INVOKER ")
	}
	b.WriteString("VIEW ")
	b.WriteString(renderQualified(view.Schema, view.Name))
	if len(view.ColumnAliases) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(view.ColumnAliases, ", "))
		b.WriteString(")")
	}
	b.WriteString(" AS\n    ")
	b.WriteString(view.Query)
	if view.CheckOption != "" {
		b.WriteString("\nWITH ")
		b.WriteString(strings.ToUpper(view.CheckOption))
		b.WriteString(" CHECK OPTION")
	}
	b.WriteString(";")
	return b.String()
}

func renderMaterializedView(view pgschema.MaterializedView) string {
	var b strings.Builder
	b.WriteString("CREATE MATERIALIZED VIEW ")
	b.WriteString(renderQualified(view.Schema, view.Name))
	if len(view.ColumnAliases) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(view.ColumnAliases, ", "))
		b.WriteString(")")
	}
	b.WriteString(" AS\n    ")
	b.WriteString(view.Query)
	if len(view.With) > 0 {
		b.WriteString("\nWITH (")
		b.WriteString(renderIndexWith(view.With))
		b.WriteString(")")
	}
	if view.NoData {
		b.WriteString("\nWITH NO DATA")
	}
	b.WriteString(";")
	return b.String()
}

func renderCreateTable(table pgschema.Table) (string, error) {
	var b strings.Builder
	b.WriteString("CREATE TABLE ")
	b.WriteString(renderTableName(table))
	if table.PartitionOf != nil {
		b.WriteString(" PARTITION OF ")
		b.WriteString(renderReferencedTable(table.PartitionOf.Parent))
		b.WriteString(" ")
		b.WriteString(renderPartitionBound(table.PartitionOf.Bound))
		if table.Partitioning != nil {
			b.WriteString("\n")
			b.WriteString(renderPartitioning(*table.Partitioning))
		}
		b.WriteString(";")
		return b.String(), nil
	}
	b.WriteString(" (\n")

	lines := make([]string, 0, len(table.Columns)+len(table.PrimaryKeys)+len(table.UniqueConstraints)+len(table.ForeignKeys)+len(table.Checks)+len(table.Exclusions))
	for _, column := range table.Columns {
		line, err := renderColumn(column)
		if err != nil {
			return "", err
		}
		lines = append(lines, "    "+line)
	}
	for _, primaryKey := range sortedBy(table.PrimaryKeys, func(item ast.PrimaryKey) string { return item.Name }) {
		lines = append(lines, "    "+renderPrimaryKeyConstraint(primaryKey))
	}
	for _, unique := range sortedBy(table.UniqueConstraints, func(item ast.UniqueConstraint) string { return item.Name }) {
		lines = append(lines, "    "+renderUniqueConstraint(unique))
	}
	for _, foreignKey := range sortedBy(table.ForeignKeys, func(item ast.ForeignKeyConstraint) string { return item.Name }) {
		lines = append(lines, "    "+renderForeignKeyConstraint(foreignKey))
	}
	for _, check := range sortedBy(table.Checks, func(item ast.Check) string { return item.Name }) {
		lines = append(lines, "    "+renderCheckConstraint(check))
	}
	for _, exclusion := range sortedBy(table.Exclusions, func(item pgschema.ExclusionConstraint) string { return item.Name }) {
		lines = append(lines, "    "+renderExclusionConstraint(exclusion))
	}
	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n)")
	if table.Partitioning != nil {
		b.WriteString("\n")
		b.WriteString(renderPartitioning(*table.Partitioning))
	}
	b.WriteString(";")
	return b.String(), nil
}

func renderPartitioning(partitioning pgschema.Partitioning) string {
	keys := make([]string, 0, len(partitioning.Keys))
	for _, key := range partitioning.Keys {
		expression := key.Expression
		if key.IsExpression {
			expression = "(" + expression + ")"
		}
		keys = append(keys, expression)
	}
	return "PARTITION BY " + strings.ToUpper(partitioning.Strategy) + " (" + strings.Join(keys, ", ") + ")"
}

func renderPartitionBound(bound pgschema.PartitionBound) string {
	switch strings.ToLower(bound.Type) {
	case "range":
		return "FOR VALUES FROM (" + strings.Join(bound.From, ", ") + ") TO (" + strings.Join(bound.To, ", ") + ")"
	case "list":
		return "FOR VALUES IN (" + strings.Join(bound.Values, ", ") + ")"
	case "hash":
		return fmt.Sprintf("FOR VALUES WITH (modulus %d, remainder %d)", bound.Modulus, bound.Remainder)
	case "default":
		return "DEFAULT"
	default:
		return ""
	}
}

func renderColumn(column ast.Column) (string, error) {
	if strings.TrimSpace(column.Type) == "" {
		return "", fmt.Errorf("column %q must have a type", column.Name)
	}
	parts := []string{column.Name, column.Type}
	if column.Identity != nil {
		parts = append(parts, renderIdentity(column.Identity))
	} else {
		if column.PrimaryKey {
			parts = append(parts, "PRIMARY KEY")
		}
		if column.NotNull {
			parts = append(parts, "NOT NULL")
		}
		if column.Unique {
			parts = append(parts, "UNIQUE")
		}
		if column.Default != "" {
			parts = append(parts, "DEFAULT", column.Default)
		}
	}
	if column.Generated != nil {
		parts = append(parts, fmt.Sprintf("GENERATED ALWAYS AS (%s) STORED", column.Generated.As))
	}
	if column.References != nil {
		parts = append(parts, fmt.Sprintf("REFERENCES %s (%s)", renderReferencedTable(column.References.Table), column.References.Column))
		if column.References.OnDelete != "" {
			parts = append(parts, "ON DELETE", strings.ToUpper(column.References.OnDelete))
		}
		if column.References.OnUpdate != "" {
			parts = append(parts, "ON UPDATE", strings.ToUpper(column.References.OnUpdate))
		}
	}
	return strings.Join(parts, " "), nil
}

func renderPrimaryKeyConstraint(primaryKey ast.PrimaryKey) string {
	return fmt.Sprintf("CONSTRAINT %s PRIMARY KEY (%s)", primaryKey.Name, strings.Join(primaryKey.Columns, ", "))
}

func renderUniqueConstraint(unique ast.UniqueConstraint) string {
	keyword := "UNIQUE"
	if unique.NullsNotDistinct {
		keyword = "UNIQUE NULLS NOT DISTINCT"
	}
	return fmt.Sprintf("CONSTRAINT %s %s (%s)%s", unique.Name, keyword, strings.Join(unique.Columns, ", "), renderDeferrable(unique.Deferrable, unique.Initially))
}

func renderForeignKeyConstraint(foreignKey ast.ForeignKeyConstraint) string {
	line := fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)", foreignKey.Name, strings.Join(foreignKey.Columns, ", "), renderReferencedTable(foreignKey.ReferencedTable), strings.Join(foreignKey.ReferencedColumns, ", "))
	if foreignKey.OnDelete != "" {
		line += " ON DELETE " + strings.ToUpper(foreignKey.OnDelete)
	}
	if foreignKey.OnUpdate != "" {
		line += " ON UPDATE " + strings.ToUpper(foreignKey.OnUpdate)
	}
	return line + renderDeferrable(foreignKey.Deferrable, foreignKey.Initially)
}

func renderCheckConstraint(check ast.Check) string {
	return fmt.Sprintf("CONSTRAINT %s CHECK (%s)", check.Name, check.Expression)
}

func renderExclusionConstraint(exclusion pgschema.ExclusionConstraint) string {
	method := exclusion.Method
	if method == "" {
		method = "gist"
	}
	elements := make([]string, 0, len(exclusion.Elements))
	for _, element := range exclusion.Elements {
		elements = append(elements, renderExclusionElement(element))
	}
	line := fmt.Sprintf("CONSTRAINT %s EXCLUDE USING %s (%s)", exclusion.Name, method, strings.Join(elements, ", "))
	if exclusion.Where != "" {
		line += " WHERE " + exclusion.Where
	}
	return line + renderDeferrable(exclusion.Deferrable, exclusion.Initially)
}

func renderExclusionElement(element pgschema.ExclusionElement) string {
	parts := []string{element.Expression}
	if element.OpClass != "" {
		parts = append(parts, element.OpClass)
	}
	parts = append(parts, "WITH", element.Operator)
	if element.Order != "" {
		parts = append(parts, element.Order)
	}
	if element.Nulls != "" {
		parts = append(parts, "NULLS", element.Nulls)
	}
	return strings.Join(parts, " ")
}

func renderPolicy(policy pgschema.Policy) string {
	var b strings.Builder
	b.WriteString("CREATE POLICY ")
	b.WriteString(policy.Name)
	b.WriteString(" ON ")
	b.WriteString(renderReferencedTable(policy.Table))
	if policy.Mode != "" {
		b.WriteString("\nAS ")
		b.WriteString(strings.ToUpper(policy.Mode))
	}
	if policy.Command != "" {
		b.WriteString("\nFOR ")
		b.WriteString(strings.ToUpper(policy.Command))
	}
	if len(policy.Roles) > 0 {
		b.WriteString("\nTO ")
		b.WriteString(strings.Join(sortedStrings(policy.Roles), ", "))
	}
	if policy.Using != "" {
		b.WriteString("\nUSING (")
		b.WriteString(policy.Using)
		b.WriteString(")")
	}
	if policy.WithCheck != "" {
		b.WriteString("\nWITH CHECK (")
		b.WriteString(policy.WithCheck)
		b.WriteString(")")
	}
	b.WriteString(";")
	return b.String()
}

func renderIndex(table pgschema.Table, index pgschema.Index) (string, error) {
	if strings.TrimSpace(index.Name) == "" {
		return "", fmt.Errorf("index on table %s must have a name", tableKey(table))
	}
	if len(index.Columns) == 0 {
		return "", fmt.Errorf("index %q must have at least one column", index.Name)
	}

	var line strings.Builder
	line.WriteString("CREATE ")
	if index.Unique {
		line.WriteString("UNIQUE ")
	}
	line.WriteString("INDEX ")
	if index.Concurrently {
		line.WriteString("CONCURRENTLY ")
	}
	line.WriteString(index.Name)
	line.WriteString(" ON ")
	if index.Only {
		line.WriteString("ONLY ")
	}
	line.WriteString(renderTableName(table))
	if index.Method != "" {
		line.WriteString(" USING ")
		line.WriteString(index.Method)
	}
	line.WriteString(" (")
	columns := make([]string, 0, len(index.Columns))
	for _, column := range index.Columns {
		columns = append(columns, renderIndexColumn(column))
	}
	line.WriteString(strings.Join(columns, ", "))
	line.WriteString(")")
	if len(index.With) > 0 {
		line.WriteString(" WITH (")
		line.WriteString(renderIndexWith(index.With))
		line.WriteString(")")
	}
	if index.Where != "" {
		line.WriteString(" WHERE ")
		line.WriteString(index.Where)
	}
	line.WriteString(";")
	return line.String(), nil
}

func renderIndexColumn(column pgschema.IndexColumn) string {
	expression := column.Expression
	if column.IsExpression {
		expression = "(" + expression + ")"
	}
	parts := []string{expression}
	if column.OpClass != "" {
		parts = append(parts, column.OpClass)
	}
	if column.Order != "" {
		parts = append(parts, column.Order)
	}
	if column.Nulls != "" {
		parts = append(parts, "NULLS", column.Nulls)
	}
	return strings.Join(parts, " ")
}

func renderIndexWith(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+" = "+values[key])
	}
	return strings.Join(parts, ", ")
}

func renderIdentity(identity *ast.Identity) string {
	keyword := "GENERATED ALWAYS AS IDENTITY"
	if strings.EqualFold(identity.Type, "bydefault") {
		keyword = "GENERATED BY DEFAULT AS IDENTITY"
	}
	var opts []string
	if identity.Name != "" {
		opts = append(opts, "SEQUENCE NAME "+identity.Name)
	}
	if identity.Increment != 0 {
		opts = append(opts, fmt.Sprintf("INCREMENT %d", identity.Increment))
	}
	if identity.MinValue != nil {
		opts = append(opts, fmt.Sprintf("MINVALUE %d", *identity.MinValue))
	}
	if identity.MaxValue != nil {
		opts = append(opts, fmt.Sprintf("MAXVALUE %d", *identity.MaxValue))
	}
	if identity.StartWith != nil {
		opts = append(opts, fmt.Sprintf("START %d", *identity.StartWith))
	}
	if identity.Cache != nil {
		opts = append(opts, fmt.Sprintf("CACHE %d", *identity.Cache))
	}
	if identity.Cycle {
		opts = append(opts, "CYCLE")
	}
	if len(opts) > 0 {
		return keyword + " (" + strings.Join(opts, " ") + ")"
	}
	return keyword
}

func renderDeferrable(deferrable bool, initially string) string {
	if !deferrable && initially == "" {
		return ""
	}
	out := " DEFERRABLE"
	if initially != "" {
		out += " INITIALLY " + strings.ToUpper(initially)
	}
	return out
}

func renderTableName(table pgschema.Table) string {
	return renderQualified(table.Schema, table.Name)
}

func renderReferencedTable(table string) string {
	if strings.Contains(table, ".") {
		return table
	}
	return table
}

func renderQualified(schema, name string) string {
	if schema == "" || schema == "public" {
		return name
	}
	return schema + "." + name
}

func renderRenameTable(oldSchema, oldName, newName string) string {
	return "ALTER TABLE " + renderQualified(oldSchema, oldName) + " RENAME TO " + newName + ";"
}

func renderReverseRenameTable(oldSchema, newName, oldName string) string {
	return "ALTER TABLE " + renderQualified(oldSchema, newName) + " RENAME TO " + oldName + ";"
}

func renderRenameColumn(schema, table, oldColumn, newColumn string) string {
	return "ALTER TABLE " + renderQualified(schema, table) + " RENAME COLUMN " + oldColumn + " TO " + newColumn + ";"
}

func renderReverseRenameColumn(schema, table, newColumn, oldColumn string) string {
	return "ALTER TABLE " + renderQualified(schema, table) + " RENAME COLUMN " + newColumn + " TO " + oldColumn + ";"
}

func renderRenameConstraint(schema, table, oldName, newName string) string {
	return "ALTER TABLE " + renderQualified(schema, table) + " RENAME CONSTRAINT " + oldName + " TO " + newName + ";"
}

func renderReverseRenameConstraint(schema, table, newName, oldName string) string {
	return "ALTER TABLE " + renderQualified(schema, table) + " RENAME CONSTRAINT " + newName + " TO " + oldName + ";"
}

func renderRenameIndex(oldSchema, oldName, newName string) string {
	return "ALTER INDEX " + renderQualified(oldSchema, oldName) + " RENAME TO " + newName + ";"
}

func renderReverseRenameIndex(oldSchema, newName, oldName string) string {
	return "ALTER INDEX " + renderQualified(oldSchema, newName) + " RENAME TO " + oldName + ";"
}

func renderRenameType(oldSchema, oldName, newName string) string {
	return "ALTER TYPE " + renderQualified(oldSchema, oldName) + " RENAME TO " + newName + ";"
}

func renderReverseRenameType(oldSchema, newName, oldName string) string {
	return "ALTER TYPE " + renderQualified(oldSchema, newName) + " RENAME TO " + oldName + ";"
}

func renderRenameSequence(oldSchema, oldName, newName string) string {
	return "ALTER SEQUENCE " + renderQualified(oldSchema, oldName) + " RENAME TO " + newName + ";"
}

func renderReverseRenameSequence(oldSchema, newName, oldName string) string {
	return "ALTER SEQUENCE " + renderQualified(oldSchema, newName) + " RENAME TO " + oldName + ";"
}

func renderRenameView(oldSchema, oldName, newName string) string {
	return "ALTER VIEW " + renderQualified(oldSchema, oldName) + " RENAME TO " + newName + ";"
}

func renderReverseRenameView(oldSchema, newName, oldName string) string {
	return "ALTER VIEW " + renderQualified(oldSchema, newName) + " RENAME TO " + oldName + ";"
}

func renderRenameMaterializedView(oldSchema, oldName, newName string) string {
	return "ALTER MATERIALIZED VIEW " + renderQualified(oldSchema, oldName) + " RENAME TO " + newName + ";"
}

func renderReverseRenameMaterializedView(oldSchema, newName, oldName string) string {
	return "ALTER MATERIALIZED VIEW " + renderQualified(oldSchema, newName) + " RENAME TO " + oldName + ";"
}

func renderRenameFunction(oldSchema, oldName, newName, identityArgs string) string {
	return "ALTER FUNCTION " + renderQualified(oldSchema, oldName) + "(" + identityArgs + ") RENAME TO " + newName + ";"
}

func renderReverseRenameFunction(oldSchema, newName, oldName, identityArgs string) string {
	return "ALTER FUNCTION " + renderQualified(oldSchema, newName) + "(" + identityArgs + ") RENAME TO " + oldName + ";"
}

func renderRenamePolicy(table, oldName, newName string) string {
	return "ALTER POLICY " + oldName + " ON " + renderReferencedTable(table) + " RENAME TO " + newName + ";"
}

func renderReverseRenamePolicy(table, newName, oldName string) string {
	return "ALTER POLICY " + newName + " ON " + renderReferencedTable(table) + " RENAME TO " + oldName + ";"
}

func renderRenameTrigger(target, oldName, newName string) string {
	return "ALTER TRIGGER " + oldName + " ON " + renderReferencedTable(target) + " RENAME TO " + newName + ";"
}

func renderReverseRenameTrigger(target, newName, oldName string) string {
	return "ALTER TRIGGER " + newName + " ON " + renderReferencedTable(target) + " RENAME TO " + oldName + ";"
}

func renderRenameRole(oldName, newName string) string {
	return "ALTER ROLE " + oldName + " RENAME TO " + newName + ";"
}

func renderReverseRenameRole(newName, oldName string) string {
	return "ALTER ROLE " + newName + " RENAME TO " + oldName + ";"
}

func renderRenameSchema(oldName, newName string) string {
	return "ALTER SCHEMA " + oldName + " RENAME TO " + newName + ";"
}

func renderReverseRenameSchema(newName, oldName string) string {
	return "ALTER SCHEMA " + newName + " RENAME TO " + oldName + ";"
}
