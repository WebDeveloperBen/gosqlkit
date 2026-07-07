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

func renderCreateTable(table ast.Table) (string, error) {
	var b strings.Builder
	b.WriteString("CREATE TABLE ")
	b.WriteString(renderTableName(table))
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
	for _, exclusion := range sortedBy(table.Exclusions, func(item ast.ExclusionConstraint) string { return item.Name }) {
		lines = append(lines, "    "+renderExclusionConstraint(exclusion))
	}
	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n);")
	return b.String(), nil
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

func renderExclusionConstraint(exclusion ast.ExclusionConstraint) string {
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

func renderExclusionElement(element ast.ExclusionElement) string {
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

func renderIndex(table ast.Table, index ast.Index) (string, error) {
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

func renderIndexColumn(column ast.IndexColumn) string {
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
	if identity.Type == "by_default" {
		keyword = "GENERATED BY DEFAULT AS IDENTITY"
	}
	var opts []string
	if identity.Name != "" {
		opts = append(opts, "SEQUENCE NAME "+identity.Name)
	}
	if identity.Increment != 0 {
		opts = append(opts, fmt.Sprintf("INCREMENT BY %d", identity.Increment))
	}
	if identity.MinValue != nil {
		opts = append(opts, fmt.Sprintf("MINVALUE %d", *identity.MinValue))
	}
	if identity.MaxValue != nil {
		opts = append(opts, fmt.Sprintf("MAXVALUE %d", *identity.MaxValue))
	}
	if identity.StartWith != nil {
		opts = append(opts, fmt.Sprintf("START WITH %d", *identity.StartWith))
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

func renderTableName(table ast.Table) string {
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
