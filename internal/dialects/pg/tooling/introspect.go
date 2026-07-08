package tooling

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

func Introspect(ctx context.Context, queryer Queryer) (pgschema.Schema, error) {
	if queryer == nil {
		return pgschema.Schema{}, errors.New("postgres queryer is required")
	}
	var schema pgschema.Schema
	var err error
	if schema.Namespaces, err = introspectNamespaces(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.Extensions, err = introspectExtensions(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.Roles, err = introspectRoles(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.Enums, err = introspectEnums(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.CompositeTypes, err = introspectCompositeTypes(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.Domains, err = introspectDomains(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.Sequences, err = introspectSequences(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.Functions, err = introspectFunctions(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.Tables, err = introspectTables(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if err := introspectColumns(ctx, queryer, schema.Tables); err != nil {
		return pgschema.Schema{}, err
	}
	if err := introspectConstraints(ctx, queryer, schema.Tables); err != nil {
		return pgschema.Schema{}, err
	}
	if err := introspectIndexes(ctx, queryer, schema.Tables); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.Views, schema.MaterializedViews, err = introspectViews(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.Policies, err = introspectPolicies(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	if schema.Triggers, err = introspectTriggers(ctx, queryer); err != nil {
		return pgschema.Schema{}, err
	}
	return schema, nil
}

func introspectNamespaces(ctx context.Context, queryer Queryer) ([]pgschema.Namespace, error) {
	rows, err := queryer.Query(ctx, `
SELECT nspname
FROM pg_namespace
WHERE nspname NOT LIKE 'pg_%'
  AND nspname <> 'information_schema'
  AND nspname <> 'public'
ORDER BY nspname`)
	if err != nil {
		return nil, fmt.Errorf("introspect namespaces: %w", err)
	}
	defer rows.Close()

	var out []pgschema.Namespace
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan namespace: %w", err)
		}
		out = append(out, pgschema.Namespace{Name: name})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect namespaces: %w", err)
	}
	return out, nil
}

func introspectExtensions(ctx context.Context, queryer Queryer) ([]pgschema.Extension, error) {
	rows, err := queryer.Query(ctx, `
SELECT e.extname, n.nspname
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
WHERE e.extname <> 'plpgsql'
ORDER BY n.nspname, e.extname`)
	if err != nil {
		return nil, fmt.Errorf("introspect extensions: %w", err)
	}
	defer rows.Close()

	var out []pgschema.Extension
	for rows.Next() {
		var item pgschema.Extension
		if err := rows.Scan(&item.Name, &item.Schema); err != nil {
			return nil, fmt.Errorf("scan extension: %w", err)
		}
		item.Schema = snapshotSchema(item.Schema)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect extensions: %w", err)
	}
	return out, nil
}

func introspectRoles(ctx context.Context, queryer Queryer) ([]pgschema.Role, error) {
	rows, err := queryer.Query(ctx, `
SELECT rolname,
       rolcanlogin,
       rolsuper,
       rolcreatedb,
       rolcreaterole,
       rolinherit,
       rolreplication,
       rolbypassrls,
       rolconnlimit,
       CASE WHEN rolvaliduntil IS NULL THEN '' ELSE rolvaliduntil::text END
FROM pg_roles
WHERE rolname NOT LIKE 'pg_%'
  AND rolname <> 'postgres'
ORDER BY rolname`)
	if err != nil {
		return nil, fmt.Errorf("introspect roles: %w", err)
	}
	defer rows.Close()

	var out []pgschema.Role
	for rows.Next() {
		var role pgschema.Role
		var login, superuser, createDB, createRole, inherit, replication, bypassRLS bool
		var connectionLimit int
		if err := rows.Scan(&role.Name, &login, &superuser, &createDB, &createRole, &inherit, &replication, &bypassRLS, &connectionLimit, &role.ValidUntil); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		role.Login = &login
		role.Superuser = &superuser
		role.CreateDB = &createDB
		role.CreateRole = &createRole
		role.Inherit = &inherit
		role.Replication = &replication
		role.BypassRLS = &bypassRLS
		role.ConnectionLimit = &connectionLimit
		out = append(out, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect roles: %w", err)
	}
	if err := introspectRoleMemberships(ctx, queryer, out); err != nil {
		return nil, err
	}
	return out, nil
}

func introspectRoleMemberships(ctx context.Context, queryer Queryer, roles []pgschema.Role) error {
	byName := make(map[string]*pgschema.Role, len(roles))
	for i := range roles {
		byName[roles[i].Name] = &roles[i]
	}
	rows, err := queryer.Query(ctx, `
SELECT member.rolname,
       parent.rolname,
       m.admin_option
FROM pg_auth_members m
JOIN pg_roles member ON member.oid = m.member
JOIN pg_roles parent ON parent.oid = m.roleid
WHERE member.rolname NOT LIKE 'pg_%'
  AND parent.rolname NOT LIKE 'pg_%'
  AND member.rolname <> 'postgres'
  AND parent.rolname <> 'postgres'
ORDER BY member.rolname, parent.rolname`)
	if err != nil {
		return fmt.Errorf("introspect role memberships: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var member, parent string
		var admin bool
		if err := rows.Scan(&member, &parent, &admin); err != nil {
			return fmt.Errorf("scan role membership: %w", err)
		}
		role := byName[member]
		if role == nil {
			continue
		}
		role.MemberOf = append(role.MemberOf, parent)
		if admin {
			role.AdminOf = append(role.AdminOf, parent)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("introspect role memberships: %w", err)
	}
	return nil
}

func introspectEnums(ctx context.Context, queryer Queryer) ([]pgschema.Enum, error) {
	rows, err := queryer.Query(ctx, `
SELECT n.nspname, t.typname, array_agg(e.enumlabel ORDER BY e.enumsortorder)::text[]
FROM pg_type t
JOIN pg_enum e ON e.enumtypid = t.oid
JOIN pg_namespace n ON n.oid = t.typnamespace
WHERE n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
GROUP BY n.nspname, t.typname
ORDER BY n.nspname, t.typname`)
	if err != nil {
		return nil, fmt.Errorf("introspect enums: %w", err)
	}
	defer rows.Close()

	var out []pgschema.Enum
	for rows.Next() {
		var item pgschema.Enum
		if err := rows.Scan(&item.Schema, &item.Name, &item.Values); err != nil {
			return nil, fmt.Errorf("scan enum: %w", err)
		}
		item.Schema = snapshotSchema(item.Schema)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect enums: %w", err)
	}
	return out, nil
}

func introspectCompositeTypes(ctx context.Context, queryer Queryer) ([]pgschema.CompositeType, error) {
	rows, err := queryer.Query(ctx, `
SELECT n.nspname,
       t.typname,
       a.attname,
       pg_catalog.format_type(a.atttypid, a.atttypmod)
FROM pg_type t
JOIN pg_class c ON c.oid = t.typrelid AND c.relkind = 'c'
JOIN pg_namespace n ON n.oid = t.typnamespace
JOIN pg_attribute a ON a.attrelid = c.oid
WHERE n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY n.nspname, t.typname, a.attnum`)
	if err != nil {
		return nil, fmt.Errorf("introspect composite types: %w", err)
	}
	defer rows.Close()

	var out []pgschema.CompositeType
	byKey := map[string]int{}
	for rows.Next() {
		var schemaName, typeName string
		var attr pgschema.CompositeAttribute
		if err := rows.Scan(&schemaName, &typeName, &attr.Name, &attr.Type); err != nil {
			return nil, fmt.Errorf("scan composite type: %w", err)
		}
		attr.Type = normaliseType(attr.Type)
		key := qualified(snapshotSchema(schemaName), typeName)
		index, ok := byKey[key]
		if !ok {
			index = len(out)
			byKey[key] = index
			out = append(out, pgschema.CompositeType{Schema: snapshotSchema(schemaName), Name: typeName})
		}
		out[index].Attributes = append(out[index].Attributes, attr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect composite types: %w", err)
	}
	return out, nil
}

func introspectDomains(ctx context.Context, queryer Queryer) ([]pgschema.Domain, error) {
	rows, err := queryer.Query(ctx, `
SELECT n.nspname,
       t.typname,
       pg_catalog.format_type(t.typbasetype, t.typtypmod),
       COALESCE(t.typdefault, ''),
       t.typnotnull,
       COALESCE(array_agg(pg_get_expr(con.conbin, 0) ORDER BY con.conname) FILTER (WHERE con.oid IS NOT NULL), ARRAY[]::text[])
FROM pg_type t
JOIN pg_namespace n ON n.oid = t.typnamespace
LEFT JOIN pg_constraint con ON con.contypid = t.oid AND con.contype = 'c'
WHERE t.typtype = 'd'
  AND n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
GROUP BY n.nspname, t.typname, t.typbasetype, t.typtypmod, t.typdefault, t.typnotnull
ORDER BY n.nspname, t.typname`)
	if err != nil {
		return nil, fmt.Errorf("introspect domains: %w", err)
	}
	defer rows.Close()

	var out []pgschema.Domain
	for rows.Next() {
		var item pgschema.Domain
		var checks []string
		if err := rows.Scan(&item.Schema, &item.Name, &item.BaseType, &item.Default, &item.NotNull, &checks); err != nil {
			return nil, fmt.Errorf("scan domain: %w", err)
		}
		item.Schema = snapshotSchema(item.Schema)
		item.BaseType = normaliseType(item.BaseType)
		for i := range checks {
			checks[i] = stripOuterParens(checks[i])
		}
		item.Check = strings.Join(checks, " AND ")
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect domains: %w", err)
	}
	return out, nil
}

func introspectSequences(ctx context.Context, queryer Queryer) ([]pgschema.Sequence, error) {
	rows, err := queryer.Query(ctx, `
SELECT n.nspname,
       c.relname,
       s.seqincrement,
       s.seqmin,
       s.seqmax,
       s.seqstart,
       s.seqcache,
       s.seqcycle,
       COALESCE(onsp.nspname, ''),
       COALESCE(oc.relname, ''),
       COALESCE(oa.attname, '')
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_sequence s ON s.seqrelid = c.oid
LEFT JOIN pg_depend dep ON dep.classid = 'pg_class'::regclass
  AND dep.objid = c.oid
  AND dep.deptype = 'a'
LEFT JOIN pg_class oc ON oc.oid = dep.refobjid
LEFT JOIN pg_namespace onsp ON onsp.oid = oc.relnamespace
LEFT JOIN pg_attribute oa ON oa.attrelid = dep.refobjid AND oa.attnum = dep.refobjsubid
WHERE c.relkind = 'S'
  AND n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
  AND NOT EXISTS (
    SELECT 1
    FROM pg_depend identity_dep
    WHERE identity_dep.classid = 'pg_class'::regclass
      AND identity_dep.objid = c.oid
      AND identity_dep.deptype = 'i'
  )
ORDER BY n.nspname, c.relname`)
	if err != nil {
		return nil, fmt.Errorf("introspect sequences: %w", err)
	}
	defer rows.Close()

	var out []pgschema.Sequence
	for rows.Next() {
		var item pgschema.Sequence
		var minValue, maxValue, startWith, cache int64
		var ownerSchema, ownerTable, ownerColumn string
		if err := rows.Scan(&item.Schema, &item.Name, &item.Increment, &minValue, &maxValue, &startWith, &cache, &item.Cycle, &ownerSchema, &ownerTable, &ownerColumn); err != nil {
			return nil, fmt.Errorf("scan sequence: %w", err)
		}
		item.Schema = snapshotSchema(item.Schema)
		item.MinValue = &minValue
		item.MaxValue = &maxValue
		item.StartWith = &startWith
		item.Cache = &cache
		if ownerTable != "" && ownerColumn != "" {
			item.OwnedBy = renderReference(snapshotSchema(ownerSchema), ownerTable) + "." + ownerColumn
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect sequences: %w", err)
	}
	return out, nil
}

func introspectFunctions(ctx context.Context, queryer Queryer) ([]pgschema.Function, error) {
	rows, err := queryer.Query(ctx, `
SELECT n.nspname,
       p.proname,
       l.lanname,
       pg_catalog.format_type(p.prorettype, NULL),
       p.prosrc,
       p.provolatile,
       p.proparallel,
       p.prosecdef,
       p.proisstrict,
       p.procost::float8,
       p.prorows::float8,
       pg_get_function_arguments(p.oid),
       COALESCE(obj_description(p.oid, 'pg_proc'), ''),
       COALESCE(p.proconfig, ARRAY[]::text[])
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
JOIN pg_language l ON l.oid = p.prolang
WHERE p.prokind = 'f'
  AND n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
  AND NOT EXISTS (
    SELECT 1
    FROM pg_depend dep
    WHERE dep.classid = 'pg_proc'::regclass
      AND dep.objid = p.oid
      AND dep.deptype = 'e'
  )
ORDER BY n.nspname, p.proname, pg_get_function_identity_arguments(p.oid)`)
	if err != nil {
		return nil, fmt.Errorf("introspect functions: %w", err)
	}
	defer rows.Close()

	var out []pgschema.Function
	for rows.Next() {
		var fn pgschema.Function
		var volatility, parallel, args string
		var strict bool
		var cost, rowsValue float64
		var config []string
		if err := rows.Scan(&fn.Schema, &fn.Name, &fn.Language, &fn.ReturnType, &fn.Body, &volatility, &parallel, &fn.SecurityDefiner, &strict, &cost, &rowsValue, &args, &fn.Comment, &config); err != nil {
			return nil, fmt.Errorf("scan function: %w", err)
		}
		fn.Schema = snapshotSchema(fn.Schema)
		fn.ReturnType = normaliseType(fn.ReturnType)
		fn.Body = normaliseDefinition(fn.Body)
		fn.Volatility = volatilityName(volatility)
		fn.Parallel = parallelName(parallel)
		fn.Strict = &strict
		fn.Cost = &cost
		rowsInt := int64(rowsValue)
		fn.Rows = &rowsInt
		fn.Arguments = parseFunctionArguments(args)
		fn.Configuration = reloptionsMap(config)
		out = append(out, fn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect functions: %w", err)
	}
	return out, nil
}

func introspectTables(ctx context.Context, queryer Queryer) ([]ast.Table, error) {
	rows, err := queryer.Query(ctx, `
SELECT n.nspname,
       c.relname,
       COALESCE(obj_description(c.oid, 'pg_class'), ''),
       c.relrowsecurity,
       c.relforcerowsecurity
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p')
  AND n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname`)
	if err != nil {
		return nil, fmt.Errorf("introspect tables: %w", err)
	}
	defer rows.Close()

	var out []ast.Table
	for rows.Next() {
		var table ast.Table
		if err := rows.Scan(&table.Schema, &table.Name, &table.Comment, &table.RowLevelSecurity, &table.ForceRLS); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		table.Schema = snapshotSchema(table.Schema)
		out = append(out, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect tables: %w", err)
	}
	return out, nil
}

func introspectColumns(ctx context.Context, queryer Queryer, tables []ast.Table) error {
	byKey := tablePointers(tables)
	rows, err := queryer.Query(ctx, `
SELECT n.nspname,
       c.relname,
       a.attname,
       pg_catalog.format_type(a.atttypid, a.atttypmod),
       a.attnotnull,
       COALESCE(pg_get_expr(ad.adbin, ad.adrelid), ''),
       COALESCE(col_description(a.attrelid, a.attnum), ''),
       CASE a.attidentity WHEN 'a' THEN 'a' WHEN 'd' THEN 'd' ELSE '' END,
       CASE a.attgenerated WHEN 's' THEN 's' ELSE '' END
FROM pg_attribute a
JOIN pg_class c ON c.oid = a.attrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
WHERE c.relkind IN ('r', 'p')
  AND n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY n.nspname, c.relname, a.attnum`)
	if err != nil {
		return fmt.Errorf("introspect columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName, identity, generated string
		var column ast.Column
		if err := rows.Scan(&schemaName, &tableName, &column.Name, &column.Type, &column.NotNull, &column.Default, &column.Comment, &identity, &generated); err != nil {
			return fmt.Errorf("scan column: %w", err)
		}
		table := byKey[qualified(snapshotSchema(schemaName), tableName)]
		if table == nil {
			continue
		}
		identity = strings.TrimSpace(identity)
		generated = strings.TrimSpace(generated)
		column.Type = normaliseType(column.Type)
		if generated != "" {
			column.Generated = &ast.Generated{As: stripOuterParens(column.Default), Type: "stored"}
			column.Default = ""
		}
		if identity != "" {
			column.Identity = &ast.Identity{Type: identityType(identity)}
			column.Default = ""
		}
		table.Columns = append(table.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("introspect columns: %w", err)
	}
	return nil
}

func introspectConstraints(ctx context.Context, queryer Queryer, tables []ast.Table) error {
	byKey := tablePointers(tables)
	rows, err := queryer.Query(ctx, `
SELECT n.nspname,
       c.relname,
       con.conname,
       con.contype,
       COALESCE((
         SELECT array_agg(a.attname ORDER BY keys.ord)::text[]
         FROM unnest(con.conkey) WITH ORDINALITY AS keys(attnum, ord)
         JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = keys.attnum
       ), ARRAY[]::text[]),
       COALESCE(fn.nspname, ''),
       COALESCE(fc.relname, ''),
       COALESCE((
         SELECT array_agg(a.attname ORDER BY keys.ord)::text[]
         FROM unnest(con.confkey) WITH ORDINALITY AS keys(attnum, ord)
         JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = keys.attnum
       ), ARRAY[]::text[]),
       con.confupdtype::text,
       con.confdeltype::text,
       con.condeferrable,
       con.condeferred,
       COALESCE(pg_get_expr(con.conbin, con.conrelid), '')
FROM pg_constraint con
JOIN pg_class c ON c.oid = con.conrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_class fc ON fc.oid = con.confrelid
LEFT JOIN pg_namespace fn ON fn.oid = fc.relnamespace
WHERE con.contype IN ('p', 'u', 'f', 'c')
  AND n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname, con.conname`)
	if err != nil {
		return fmt.Errorf("introspect constraints: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName, name, kind, refSchema, refTable, onUpdate, onDelete, checkExpr string
		var columns, refColumns []string
		var deferrable, deferred bool
		if err := rows.Scan(&schemaName, &tableName, &name, &kind, &columns, &refSchema, &refTable, &refColumns, &onUpdate, &onDelete, &deferrable, &deferred, &checkExpr); err != nil {
			return fmt.Errorf("scan constraint: %w", err)
		}
		table := byKey[qualified(snapshotSchema(schemaName), tableName)]
		if table == nil {
			continue
		}
		switch kind {
		case "p":
			addPrimaryKey(table, name, columns)
		case "u":
			addUnique(table, name, columns, deferrable, deferred)
		case "f":
			addForeignKey(table, name, columns, snapshotSchema(refSchema), refTable, refColumns, onUpdate, onDelete, deferrable, deferred)
		case "c":
			table.Checks = append(table.Checks, ast.Check{Name: name, Expression: stripOuterParens(checkExpr)})
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("introspect constraints: %w", err)
	}
	return nil
}

func introspectIndexes(ctx context.Context, queryer Queryer, tables []ast.Table) error {
	byKey := tablePointers(tables)
	rows, err := queryer.Query(ctx, `
SELECT n.nspname,
       c.relname,
       idx.relname,
       i.indisunique,
       am.amname,
       COALESCE(pg_get_expr(i.indpred, i.indrelid), ''),
       COALESCE(idx.reloptions, ARRAY[]::text[]),
       keydef.def,
       keydef.is_expression,
       keydef.option
FROM pg_index i
JOIN pg_class idx ON idx.oid = i.indexrelid
JOIN pg_class c ON c.oid = i.indrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_am am ON am.oid = idx.relam
JOIN LATERAL (
  SELECT keys.ord,
         pg_get_indexdef(i.indexrelid, keys.ord::int, true) AS def,
         keys.attnum = 0 AS is_expression,
         i.indoption[keys.ord::int - 1] AS option
  FROM unnest(i.indkey) WITH ORDINALITY AS keys(attnum, ord)
  WHERE keys.ord <= i.indnkeyatts
) keydef ON true
WHERE n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
  AND NOT EXISTS (
    SELECT 1
    FROM pg_constraint con
    WHERE con.conindid = i.indexrelid
  )
ORDER BY n.nspname, c.relname, idx.relname, keydef.ord`)
	if err != nil {
		return fmt.Errorf("introspect indexes: %w", err)
	}
	defer rows.Close()

	var currentTable, currentName string
	var current *ast.Index
	for rows.Next() {
		var schemaName, tableName, name, method, predicate, definition string
		var reloptions []string
		var unique, expression bool
		var option int
		if err := rows.Scan(&schemaName, &tableName, &name, &unique, &method, &predicate, &reloptions, &definition, &expression, &option); err != nil {
			return fmt.Errorf("scan index: %w", err)
		}
		table := byKey[qualified(snapshotSchema(schemaName), tableName)]
		if table == nil {
			continue
		}
		key := qualified(snapshotSchema(schemaName), tableName) + "." + name
		if current == nil || currentTable != key || currentName != name {
			table.Indexes = append(table.Indexes, ast.Index{
				Name:    name,
				Method:  method,
				Where:   stripOuterParens(predicate),
				With:    reloptionsMap(reloptions),
				Unique:  unique,
				Columns: []ast.IndexColumn{},
			})
			current = &table.Indexes[len(table.Indexes)-1]
			currentTable = key
			currentName = name
		}
		current.Columns = append(current.Columns, parseIndexColumn(definition, expression, option))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("introspect indexes: %w", err)
	}
	return nil
}

func introspectViews(ctx context.Context, queryer Queryer) ([]pgschema.View, []pgschema.MaterializedView, error) {
	rows, err := queryer.Query(ctx, `
SELECT n.nspname,
       c.relname,
       c.relkind,
       pg_get_viewdef(c.oid, true),
       COALESCE(obj_description(c.oid, 'pg_class'), ''),
       COALESCE(c.reloptions, ARRAY[]::text[]),
       c.relispopulated,
       COALESCE(v.check_option, 'NONE')
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN information_schema.views v ON v.table_schema = n.nspname AND v.table_name = c.relname
WHERE c.relkind IN ('v', 'm')
  AND n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
  AND NOT EXISTS (
    SELECT 1
    FROM pg_depend dep
    WHERE dep.classid = 'pg_class'::regclass
      AND dep.objid = c.oid
      AND dep.deptype = 'e'
  )
ORDER BY n.nspname, c.relname`)
	if err != nil {
		return nil, nil, fmt.Errorf("introspect views: %w", err)
	}
	defer rows.Close()

	var views []pgschema.View
	var materializedViews []pgschema.MaterializedView
	for rows.Next() {
		var schemaName, name, kind, query, comment, checkOption string
		var reloptions []string
		var populated bool
		if err := rows.Scan(&schemaName, &name, &kind, &query, &comment, &reloptions, &populated, &checkOption); err != nil {
			return nil, nil, fmt.Errorf("scan view: %w", err)
		}
		schemaName = snapshotSchema(schemaName)
		options := reloptionsMap(reloptions)
		if kind == "m" {
			materializedViews = append(materializedViews, pgschema.MaterializedView{
				Schema:  schemaName,
				Name:    name,
				Query:   normaliseDefinition(query),
				Comment: comment,
				With:    options,
				NoData:  !populated,
			})
			continue
		}
		view := pgschema.View{
			Schema:          schemaName,
			Name:            name,
			Query:           normaliseDefinition(query),
			Comment:         comment,
			CheckOption:     viewCheckOption(checkOption),
			SecurityBarrier: options["security_barrier"] == "true",
			SecurityInvoker: options["security_invoker"] == "true",
		}
		views = append(views, view)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("introspect views: %w", err)
	}
	return views, materializedViews, nil
}

func introspectPolicies(ctx context.Context, queryer Queryer) ([]pgschema.Policy, error) {
	rows, err := queryer.Query(ctx, `
SELECT schemaname,
       tablename,
       policyname,
       permissive,
       cmd,
       COALESCE(qual, ''),
       COALESCE(with_check, ''),
       roles::text[]
FROM pg_policies
WHERE schemaname NOT LIKE 'pg_%'
  AND schemaname <> 'information_schema'
ORDER BY schemaname, tablename, policyname`)
	if err != nil {
		return nil, fmt.Errorf("introspect policies: %w", err)
	}
	defer rows.Close()

	var out []pgschema.Policy
	for rows.Next() {
		var schemaName, tableName string
		var roles []string
		var policy pgschema.Policy
		if err := rows.Scan(&schemaName, &tableName, &policy.Name, &policy.Mode, &policy.Command, &policy.Using, &policy.WithCheck, &roles); err != nil {
			return nil, fmt.Errorf("scan policy: %w", err)
		}
		policy.Table = renderReference(snapshotSchema(schemaName), tableName)
		policy.Mode = strings.ToUpper(policy.Mode)
		policy.Command = strings.ToUpper(policy.Command)
		policy.Using = stripOuterParens(policy.Using)
		policy.WithCheck = stripOuterParens(policy.WithCheck)
		if len(roles) == 1 && roles[0] == "public" {
			roles = nil
		}
		policy.Roles = roles
		out = append(out, policy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect policies: %w", err)
	}
	return out, nil
}

func introspectTriggers(ctx context.Context, queryer Queryer) ([]pgschema.Trigger, error) {
	rows, err := queryer.Query(ctx, `
SELECT n.nspname,
       c.relname,
       t.tgname,
       t.tgtype,
       COALESCE(pn.nspname, ''),
       p.proname,
       pg_get_triggerdef(t.oid, true),
       COALESCE(obj_description(t.oid, 'pg_trigger'), ''),
       COALESCE((
         SELECT array_agg(a.attname ORDER BY a.attnum)::text[]
         FROM unnest(t.tgattr) AS attrs(attnum)
         JOIN pg_attribute a ON a.attrelid = t.tgrelid AND a.attnum = attrs.attnum
       ), ARRAY[]::text[]),
       t.tgargs,
       t.tgconstraint <> 0,
       COALESCE(con.condeferrable, false),
       COALESCE(con.condeferred, false),
       COALESCE(rn.nspname, ''),
       COALESCE(rc.relname, '')
FROM pg_trigger t
JOIN pg_class c ON c.oid = t.tgrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_proc p ON p.oid = t.tgfoid
JOIN pg_namespace pn ON pn.oid = p.pronamespace
LEFT JOIN pg_constraint con ON con.oid = t.tgconstraint
LEFT JOIN pg_class rc ON rc.oid = con.confrelid
LEFT JOIN pg_namespace rn ON rn.oid = rc.relnamespace
WHERE NOT t.tgisinternal
  AND n.nspname NOT LIKE 'pg_%'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname, t.tgname`)
	if err != nil {
		return nil, fmt.Errorf("introspect triggers: %w", err)
	}
	defer rows.Close()

	var out []pgschema.Trigger
	for rows.Next() {
		var schemaName, targetName, functionSchema, functionName, triggerDef, referencedSchema, referencedTable string
		var trigger pgschema.Trigger
		var args []byte
		var tgtype int
		var deferred bool
		if err := rows.Scan(&schemaName, &targetName, &trigger.Name, &tgtype, &functionSchema, &functionName, &triggerDef, &trigger.Comment, &trigger.Columns, &args, &trigger.Constraint, &trigger.Deferrable, &deferred, &referencedSchema, &referencedTable); err != nil {
			return nil, fmt.Errorf("scan trigger: %w", err)
		}
		trigger.Target = renderReference(snapshotSchema(schemaName), targetName)
		trigger.Function = renderReference(snapshotSchema(functionSchema), functionName)
		trigger.Timing = triggerTiming(tgtype)
		trigger.Level = triggerLevel(tgtype)
		trigger.Events = triggerEvents(tgtype)
		trigger.When = parseTriggerWhen(triggerDef)
		if trigger.Deferrable {
			trigger.Initially = initially(deferred)
		}
		if referencedTable != "" {
			trigger.ReferencedTable = renderReference(snapshotSchema(referencedSchema), referencedTable)
		}
		trigger.Arguments = parseTriggerArguments(args)
		out = append(out, trigger)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("introspect triggers: %w", err)
	}
	return out, nil
}

func tablePointers(tables []ast.Table) map[string]*ast.Table {
	out := make(map[string]*ast.Table, len(tables))
	for i := range tables {
		out[qualified(tables[i].Schema, tables[i].Name)] = &tables[i]
	}
	return out
}

func parseFunctionArguments(value string) []pgschema.FunctionArgument {
	parts := splitComma(value)
	args := make([]pgschema.FunctionArgument, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		arg := pgschema.FunctionArgument{}
		if before, after, ok := cutInsensitive(part, " DEFAULT "); ok {
			part = before
			arg.Default = strings.TrimSpace(after)
		} else if before, after, ok := cutInsensitive(part, " = "); ok {
			part = before
			arg.Default = strings.TrimSpace(after)
		}
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		if isFunctionArgMode(fields[0]) {
			arg.Mode = strings.ToUpper(fields[0])
			fields = fields[1:]
		}
		switch len(fields) {
		case 0:
			continue
		case 1:
			arg.Type = normaliseType(fields[0])
		default:
			arg.Name = fields[0]
			arg.Type = normaliseType(strings.Join(fields[1:], " "))
		}
		args = append(args, arg)
	}
	return args
}

func splitComma(value string) []string {
	var out []string
	var b strings.Builder
	var depth int
	inString := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch ch {
		case '\'':
			inString = !inString
		case '(':
			if !inString {
				depth++
			}
		case ')':
			if !inString {
				depth--
			}
		case ',':
			if !inString && depth == 0 {
				out = append(out, b.String())
				b.Reset()
				continue
			}
		}
		b.WriteByte(ch)
	}
	out = append(out, b.String())
	return out
}

func cutInsensitive(value, sep string) (string, string, bool) {
	index := strings.Index(strings.ToUpper(value), strings.ToUpper(sep))
	if index < 0 {
		return "", "", false
	}
	return value[:index], value[index+len(sep):], true
}

func isFunctionArgMode(value string) bool {
	switch strings.ToUpper(value) {
	case "IN", "OUT", "INOUT", "VARIADIC":
		return true
	default:
		return false
	}
}

func volatilityName(value string) string {
	switch value {
	case "i":
		return "IMMUTABLE"
	case "s":
		return "STABLE"
	default:
		return "VOLATILE"
	}
}

func parallelName(value string) string {
	switch value {
	case "s":
		return "SAFE"
	case "r":
		return "RESTRICTED"
	default:
		return "UNSAFE"
	}
}

func viewCheckOption(value string) string {
	switch strings.ToUpper(value) {
	case "CASCADED", "LOCAL":
		return strings.ToUpper(value)
	default:
		return ""
	}
}

func triggerTiming(tgtype int) string {
	const (
		triggerBefore  = 1 << 1
		triggerInstead = 1 << 6
	)
	switch {
	case tgtype&triggerInstead != 0:
		return "INSTEAD OF"
	case tgtype&triggerBefore != 0:
		return "BEFORE"
	default:
		return "AFTER"
	}
}

func triggerLevel(tgtype int) string {
	const triggerRow = 1 << 0
	if tgtype&triggerRow != 0 {
		return "ROW"
	}
	return "STATEMENT"
}

func triggerEvents(tgtype int) []string {
	events := []string{}
	for _, event := range []struct {
		name string
		bit  int
	}{
		{name: "INSERT", bit: 1 << 2},
		{name: "DELETE", bit: 1 << 3},
		{name: "UPDATE", bit: 1 << 4},
		{name: "TRUNCATE", bit: 1 << 5},
	} {
		if tgtype&event.bit != 0 {
			events = append(events, event.name)
		}
	}
	slices.Sort(events)
	return events
}

func parseTriggerArguments(value []byte) []string {
	parts := strings.Split(strings.TrimRight(string(value), "\x00"), "\x00")
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}
	return parts
}

func parseTriggerWhen(definition string) string {
	upper := strings.ToUpper(definition)
	whenIndex := strings.Index(upper, " WHEN ")
	if whenIndex < 0 {
		return ""
	}
	executeIndex := strings.Index(upper[whenIndex:], " EXECUTE ")
	if executeIndex < 0 {
		return ""
	}
	value := strings.TrimSpace(definition[whenIndex+len(" WHEN ") : whenIndex+executeIndex])
	return stripOuterParens(value)
}

func normaliseDefinition(value string) string {
	return strings.TrimSuffix(strings.TrimSpace(value), ";")
}

func reloptionsMap(values []string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for _, value := range values {
		key, optionValue, ok := strings.Cut(value, "=")
		if !ok {
			out[value] = ""
			continue
		}
		out[key] = optionValue
	}
	return out
}

func parseIndexColumn(definition string, expression bool, option int) ast.IndexColumn {
	out := ast.IndexColumn{Expression: strings.TrimSpace(definition), IsExpression: expression}
	out.Expression = consumeIndexSuffix(out.Expression, "NULLS FIRST", func() { out.Nulls = "FIRST" })
	out.Expression = consumeIndexSuffix(out.Expression, "NULLS LAST", func() { out.Nulls = "LAST" })
	out.Expression = consumeIndexSuffix(out.Expression, "ASC", func() { out.Order = "ASC" })
	out.Expression = consumeIndexSuffix(out.Expression, "DESC", func() { out.Order = "DESC" })
	if option&1 != 0 {
		out.Order = "DESC"
	}
	if option&2 != 0 {
		out.Nulls = "FIRST"
	} else if out.Order == "DESC" && out.Nulls == "" {
		out.Nulls = "LAST"
	}
	if !expression {
		parts := strings.Fields(out.Expression)
		if len(parts) > 1 {
			out.Expression = strings.Join(parts[:len(parts)-1], " ")
			out.OpClass = parts[len(parts)-1]
		}
	}
	out.Expression = strings.TrimSpace(out.Expression)
	return out
}

func consumeIndexSuffix(value, suffix string, fn func()) string {
	trimmed := strings.TrimSpace(value)
	if !strings.HasSuffix(trimmed, suffix) {
		return value
	}
	fn()
	return strings.TrimSpace(strings.TrimSuffix(trimmed, suffix))
}

func addPrimaryKey(table *ast.Table, name string, columns []string) {
	if len(columns) == 1 && name == table.Name+"_pkey" {
		setColumn(table, columns[0], func(column *ast.Column) { column.PrimaryKey = true })
		return
	}
	table.PrimaryKeys = append(table.PrimaryKeys, ast.PrimaryKey{Name: name, Columns: columns})
}

func addUnique(table *ast.Table, name string, columns []string, deferrable, deferred bool) {
	if len(columns) == 1 && name == table.Name+"_"+columns[0]+"_key" && !deferrable {
		setColumn(table, columns[0], func(column *ast.Column) { column.Unique = true })
		return
	}
	unique := ast.UniqueConstraint{Name: name, Columns: columns, Deferrable: deferrable}
	if deferrable {
		unique.Initially = initially(deferred)
	}
	table.UniqueConstraints = append(table.UniqueConstraints, unique)
}

func addForeignKey(table *ast.Table, name string, columns []string, refSchema, refTable string, refColumns []string, onUpdate, onDelete string, deferrable, deferred bool) {
	referencedTable := renderReference(refSchema, refTable)
	if len(columns) == 1 && len(refColumns) == 1 && name == table.Name+"_"+columns[0]+"_fkey" && !deferrable {
		setColumn(table, columns[0], func(column *ast.Column) {
			column.References = &ast.ForeignKey{
				Table:    referencedTable,
				Column:   refColumns[0],
				OnDelete: action(onDelete),
				OnUpdate: action(onUpdate),
			}
		})
		return
	}
	foreignKey := ast.ForeignKeyConstraint{
		Name:              name,
		Columns:           columns,
		ReferencedTable:   referencedTable,
		ReferencedColumns: refColumns,
		OnDelete:          action(onDelete),
		OnUpdate:          action(onUpdate),
		Deferrable:        deferrable,
	}
	if deferrable {
		foreignKey.Initially = initially(deferred)
	}
	table.ForeignKeys = append(table.ForeignKeys, foreignKey)
}

func setColumn(table *ast.Table, name string, fn func(*ast.Column)) {
	for i := range table.Columns {
		if table.Columns[i].Name == name {
			fn(&table.Columns[i])
			return
		}
	}
}

func snapshotSchema(schema string) string {
	if schema == "public" {
		return ""
	}
	return schema
}

func qualified(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}

func renderReference(schema, table string) string {
	if schema == "" || schema == "public" {
		return "public." + table
	}
	return schema + "." + table
}

func identityType(value string) string {
	if value == "d" {
		return "byDefault"
	}
	return "always"
}

func initially(deferred bool) string {
	if deferred {
		return "DEFERRED"
	}
	return "IMMEDIATE"
}

func action(value string) string {
	switch value {
	case "r":
		return "restrict"
	case "c":
		return "cascade"
	case "n":
		return "set null"
	case "d":
		return "set default"
	default:
		return ""
	}
}

func normaliseType(value string) string {
	replacements := []struct {
		old string
		new string
	}{
		{"timestamp with time zone", "timestamptz"},
		{"timestamp without time zone", "timestamp"},
		{"time with time zone", "timetz"},
		{"time without time zone", "time"},
		{"character varying", "varchar"},
		{"character", "char"},
	}
	out := value
	for _, replacement := range replacements {
		out = strings.ReplaceAll(out, replacement.old, replacement.new)
	}
	return out
}

var parenPattern = regexp.MustCompile(`^\((.*)\)$`)

func stripOuterParens(value string) string {
	out := strings.TrimSpace(value)
	for {
		matches := parenPattern.FindStringSubmatch(out)
		if len(matches) != 2 || !balanced(matches[1]) {
			return out
		}
		out = strings.TrimSpace(matches[1])
	}
}

func balanced(value string) bool {
	var depth int
	inString := false
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\'':
			inString = !inString
		case '(':
			if !inString {
				depth++
			}
		case ')':
			if !inString {
				depth--
				if depth < 0 {
					return false
				}
			}
		}
	}
	return depth == 0 && !inString
}
