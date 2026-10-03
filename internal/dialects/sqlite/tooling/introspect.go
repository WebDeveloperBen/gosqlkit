package tooling

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

// Introspect returns the modeled SQLite schema in stable name order. Catalog
// SQL is deliberately parsed conservatively: an object which cannot be
// represented must not disappear from a successful snapshot.
func Introspect(ctx context.Context, conn *Conn) (sqliteschema.Schema, error) {
	var result sqliteschema.Schema
	tables, err := conn.Query(ctx, `SELECT name, sql FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return result, fmt.Errorf("read SQLite tables: %w", err)
	}
	defer func() {
		_ = tables.Close()
	}()
	for tables.Next() {
		var name string
		var source sql.NullString
		if err := tables.Scan(&name, &source); err != nil {
			return result, fmt.Errorf("read SQLite table row: %w", err)
		}
		if migrationStateTable(name) {
			continue
		}
		if !source.Valid {
			return result, fmt.Errorf("table %q has no catalog SQL and cannot be represented", name)
		}
		t, err := introspectTable(ctx, conn, name, source.String)
		if err != nil {
			return result, err
		}
		result.Tables = append(result.Tables, t)
	}
	if err := tables.Err(); err != nil {
		return result, fmt.Errorf("read SQLite tables: %w", err)
	}
	views, err := conn.Query(ctx, `SELECT name, sql FROM sqlite_schema WHERE type='view' ORDER BY name`)
	if err != nil {
		return result, fmt.Errorf("read SQLite views: %w", err)
	}
	defer func() {
		_ = views.Close()
	}()
	for views.Next() {
		var name string
		var source sql.NullString
		if err := views.Scan(&name, &source); err != nil {
			return result, fmt.Errorf("read SQLite view row: %w", err)
		}
		if !source.Valid {
			return result, fmt.Errorf("view %q has no catalog SQL", name)
		}
		v, err := parseView(name, source.String)
		if err != nil {
			return result, err
		}
		result.Views = append(result.Views, v)
	}
	if err := views.Err(); err != nil {
		return result, fmt.Errorf("read SQLite views: %w", err)
	}
	triggers, err := conn.Query(ctx, `SELECT name, tbl_name, sql FROM sqlite_schema WHERE type='trigger' ORDER BY name`)
	if err != nil {
		return result, fmt.Errorf("read SQLite triggers: %w", err)
	}
	defer func() {
		_ = triggers.Close()
	}()
	for triggers.Next() {
		var name, target string
		var source sql.NullString
		if err := triggers.Scan(&name, &target, &source); err != nil {
			return result, fmt.Errorf("read SQLite trigger row: %w", err)
		}
		if !source.Valid {
			return result, fmt.Errorf("trigger %q on %q has no catalog SQL", name, target)
		}
		t, err := parseTrigger(name, target, source.String)
		if err != nil {
			return result, err
		}
		result.Triggers = append(result.Triggers, t)
	}
	if err := triggers.Err(); err != nil {
		return result, fmt.Errorf("read SQLite triggers: %w", err)
	}
	sort.Slice(result.Tables, func(i, j int) bool { return result.Tables[i].Name < result.Tables[j].Name })
	sort.Slice(result.Views, func(i, j int) bool { return result.Views[i].Name < result.Views[j].Name })
	sort.Slice(result.Triggers, func(i, j int) bool { return result.Triggers[i].Name < result.Triggers[j].Name })
	return result, nil
}

func migrationStateTable(name string) bool {
	return name == "goose_db_version" || name == "schema_migrations"
}

func introspectTable(ctx context.Context, conn *Conn, name, source string) (sqliteschema.Table, error) {
	t := sqliteschema.Table{Table: ast.Table{Name: name}}
	if strings.Contains(strings.ToUpper(source), "CREATE VIRTUAL TABLE") {
		return t, fmt.Errorf("table %q uses a virtual-table declaration, which SQLite schema introspection cannot model", name)
	}
	body, err := createBody(source, "TABLE", name)
	if err != nil {
		return t, fmt.Errorf("table %q: %w", name, err)
	}
	t.Strict, t.WithoutRowID, err = parseTableOptions(source, name)
	if err != nil {
		return t, fmt.Errorf("table %q: %w", name, err)
	}
	parts, err := splitSQLTop(body, ',')
	if err != nil {
		return t, fmt.Errorf("table %q: %w", name, err)
	}
	for _, part := range parts {
		s := strings.TrimSpace(part)
		if s == "" {
			return t, fmt.Errorf("table %q contains an empty definition", name)
		}
		kind := strings.ToUpper(firstWord(s))
		if kind == "CONSTRAINT" || kind == "PRIMARY" || kind == "UNIQUE" || kind == "FOREIGN" || kind == "CHECK" {
			if err := parseTableConstraint(&t, s); err != nil {
				return t, fmt.Errorf("table %q constraint %q: %w", name, s, err)
			}
			continue
		}
		col, auto, checks, err := parseColumn(s)
		if err != nil {
			return t, fmt.Errorf("table %q column %q: %w", name, firstWord(s), err)
		}
		if auto {
			col.AutoIncrement = true
		}
		t.Checks = append(t.Checks, checks...)
		t.Columns = append(t.Columns, col)
	}
	// PRAGMAs provide authoritative ordered column/key metadata; reject any
	// catalog shape which the source parser did not account for.
	if err := validateTablePragmas(ctx, conn, name, &t); err != nil {
		return t, err
	}
	if err := introspectIndexes(ctx, conn, name, &t); err != nil {
		return t, err
	}
	return t, nil
}

func parseTableOptions(source, name string) (bool, bool, error) {
	value := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(source), ";"))
	upper := strings.ToUpper(value)
	if !strings.HasPrefix(upper, "CREATE ") {
		return false, false, errors.New("unsupported CREATE TABLE statement")
	}
	position := strings.Index(upper, "TABLE")
	if position < 0 {
		return false, false, errors.New("unsupported CREATE TABLE statement")
	}
	rest := strings.TrimSpace(value[position+len("TABLE"):])
	if strings.HasPrefix(strings.ToUpper(rest), "IF NOT EXISTS") {
		rest = strings.TrimSpace(rest[len("IF NOT EXISTS"):])
	}
	tableName, rest, ok := takeWord(rest)
	if !ok || unquoteIdent(tableName) != name {
		return false, false, errors.New("catalog name does not match CREATE TABLE name")
	}
	rest = strings.TrimSpace(rest)
	if rest == "" || rest[0] != '(' {
		return false, false, errors.New("missing table definition")
	}
	end := matchingParen(rest, 0)
	if end < 0 {
		return false, false, errors.New("malformed table definition")
	}
	suffix := strings.TrimSpace(rest[end+1:])
	if suffix == "" {
		return false, false, nil
	}
	options, err := splitSQLTop(suffix, ',')
	if err != nil {
		return false, false, fmt.Errorf("malformed table options: %w", err)
	}
	strict, withoutRowID := false, false
	for _, option := range options {
		switch strings.ToUpper(strings.TrimSpace(option)) {
		case "STRICT":
			if strict {
				return false, false, errors.New("duplicate STRICT table option")
			}
			strict = true
		case "WITHOUT ROWID":
			if withoutRowID {
				return false, false, errors.New("duplicate WITHOUT ROWID table option")
			}
			withoutRowID = true
		default:
			return false, false, fmt.Errorf("unsupported table option %q", option)
		}
	}
	return strict, withoutRowID, nil
}

func validateTablePragmas(ctx context.Context, conn *Conn, name string, t *sqliteschema.Table) error {
	rows, err := conn.Query(ctx, "PRAGMA table_xinfo("+quoteIdent(name)+")")
	if err != nil {
		return fmt.Errorf("table %q columns: %w", name, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	primaryKeys := make(map[string]int)
	for _, key := range t.PrimaryKeys {
		for i, column := range key.Columns {
			if primaryKeys[column] != 0 {
				return fmt.Errorf("table %q declares column %q in multiple primary keys", name, column)
			}
			primaryKeys[column] = i + 1
		}
	}
	for _, column := range t.Columns {
		if !column.PrimaryKey {
			continue
		}
		if primaryKeys[column.Name] != 0 {
			return fmt.Errorf("table %q declares column %q as both inline and table-level primary key", name, column.Name)
		}
		primaryKeys[column.Name] = 1
	}

	n := 0
	for rows.Next() {
		var cid, notnull, pk, hidden int
		var columnName, declaredType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &columnName, &declaredType, &notnull, &defaultValue, &pk, &hidden); err != nil {
			return fmt.Errorf("table %q column metadata: %w", name, err)
		}
		if hidden == 1 {
			return fmt.Errorf("table %q column %q is hidden and cannot be modeled", name, columnName)
		}
		if cid != n || n >= len(t.Columns) || t.Columns[n].Name != columnName {
			return fmt.Errorf("table %q column %q has a catalog definition not safely recognized", name, columnName)
		}
		column := &t.Columns[n]
		column.Type = declaredType
		if notnull != 0 && !column.NotNull && (pk == 0 || (!t.WithoutRowID && !t.Strict)) {
			return fmt.Errorf("table %q column %q has an unmodeled NOT NULL property", name, columnName)
		}
		if column.NotNull && notnull == 0 {
			return fmt.Errorf("table %q column %q NOT NULL declaration disagrees with SQLite catalog metadata", name, columnName)
		}
		if primaryKeys[columnName] != pk {
			return fmt.Errorf("table %q column %q primary-key ordinal disagrees with SQLite catalog metadata", name, columnName)
		}
		if defaultValue.Valid && column.Default == "" {
			column.Default = defaultValue.String
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("table %q columns: %w", name, err)
	}
	if n != len(t.Columns) {
		return fmt.Errorf("table %q has columns not represented by its CREATE TABLE SQL", name)
	}

	fkRows, err := conn.Query(ctx, "PRAGMA foreign_key_list("+quoteIdent(name)+")")
	if err != nil {
		return fmt.Errorf("table %q foreign keys: %w", name, err)
	}
	defer func() {
		_ = fkRows.Close()
	}()
	type catalogForeignKey struct {
		table    string
		from     string
		to       string
		onUpdate string
		onDelete string
		match    string
		id       int
		sequence int
	}
	groups := make(map[int][]catalogForeignKey)
	for fkRows.Next() {
		var key catalogForeignKey
		if err := fkRows.Scan(&key.id, &key.sequence, &key.table, &key.from, &key.to, &key.onUpdate, &key.onDelete, &key.match); err != nil {
			return fmt.Errorf("table %q foreign keys: %w", name, err)
		}
		groups[key.id] = append(groups[key.id], key)
	}
	if err := fkRows.Err(); err != nil {
		return fmt.Errorf("table %q foreign keys: %w", name, err)
	}
	for id := range groups {
		sort.Slice(groups[id], func(i, j int) bool {
			return groups[id][i].sequence < groups[id][j].sequence
		})
	}

	type expectedForeignKey struct {
		table, onUpdate, onDelete string
		columns, referenced       []string
	}
	expected := make([]expectedForeignKey, 0, len(t.ForeignKeys)+len(t.Columns))
	for _, column := range t.Columns {
		if column.References == nil {
			continue
		}
		expected = append(expected, expectedForeignKey{
			table: column.References.Table, onUpdate: column.References.OnUpdate, onDelete: column.References.OnDelete,
			columns: []string{column.Name}, referenced: []string{column.References.Column},
		})
	}
	for _, key := range t.ForeignKeys {
		expected = append(expected, expectedForeignKey{
			table: key.ReferencedTable, onUpdate: key.OnUpdate, onDelete: key.OnDelete,
			columns: key.Columns, referenced: key.ReferencedColumns,
		})
	}
	if len(groups) != len(expected) {
		return fmt.Errorf("table %q foreign-key catalog contains unsupported or unrecognized constraints", name)
	}
	groupIDs := make([]int, 0, len(groups))
	for id := range groups {
		groupIDs = append(groupIDs, id)
	}
	sort.Ints(groupIDs)
	matched := make([]bool, len(groupIDs))
	for _, wanted := range expected {
		found := false
		for i, id := range groupIDs {
			if matched[i] {
				continue
			}
			actual := groups[id]
			if len(actual) != len(wanted.columns) || len(actual) != len(wanted.referenced) {
				continue
			}
			equal := true
			for j, row := range actual {
				if row.sequence != j || row.table != wanted.table || row.from != wanted.columns[j] || row.to != wanted.referenced[j] ||
					normalizedForeignKeyAction(row.onUpdate) != normalizedForeignKeyAction(wanted.onUpdate) ||
					normalizedForeignKeyAction(row.onDelete) != normalizedForeignKeyAction(wanted.onDelete) || row.match != "NONE" {
					equal = false
					break
				}
			}
			if equal {
				matched[i] = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("table %q foreign-key catalog does not match its modeled constraints", name)
		}
	}
	return nil
}

func normalizedForeignKeyAction(action string) string {
	if strings.TrimSpace(action) == "" {
		return "NO ACTION"
	}
	return strings.ToUpper(strings.Join(strings.Fields(action), " "))
}

func introspectIndexes(ctx context.Context, conn *Conn, table string, t *sqliteschema.Table) error {
	rows, err := conn.Query(ctx, "PRAGMA index_list("+quoteIdent(table)+")")
	if err != nil {
		return fmt.Errorf("table %q indexes: %w", table, err)
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return fmt.Errorf("table %q index metadata: %w", table, err)
		}
		if origin == "pk" || origin == "u" {
			continue
		}
		if strings.HasPrefix(name, "sqlite_autoindex_") {
			continue
		}
		var source sql.NullString
		if err := conn.QueryRow(ctx, "SELECT sql FROM sqlite_schema WHERE type='index' AND name=?", name).Scan(&source); err != nil {
			return fmt.Errorf("index %q on table %q catalog SQL: %w", name, table, err)
		}
		if !source.Valid {
			return fmt.Errorf("index %q on table %q has no SQL definition", name, table)
		}
		idx, err := parseIndex(name, table, source.String, unique != 0, partial != 0)
		if err != nil {
			return err
		}
		t.Indexes = append(t.Indexes, idx)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("table %q indexes: %w", table, err)
	}
	return nil
}

func parseIndex(name, table, source string, unique, partial bool) (sqliteschema.Index, error) {
	body, err := createBody(source, "INDEX", name)
	if err != nil {
		return sqliteschema.Index{}, fmt.Errorf("index %q on table %q: %w", name, table, err)
	}
	// Locate ON table and the parenthesized key list without accepting clauses
	// that the structured index model cannot preserve.
	up := strings.ToUpper(body)
	on := strings.Index(up, " ON ")
	if on < 0 {
		return sqliteschema.Index{}, fmt.Errorf("index %q on table %q has unsupported CREATE INDEX syntax", name, table)
	}
	rest := strings.TrimSpace(body[on+4:])
	if unquoteIdent(firstWord(rest)) != table {
		return sqliteschema.Index{}, fmt.Errorf("index %q targets an unsupported table", name)
	}
	open := strings.Index(rest, "(")
	if open < 0 {
		return sqliteschema.Index{}, fmt.Errorf("index %q has no key list", name)
	}
	end := matchingParen(rest, open)
	if end < 0 {
		return sqliteschema.Index{}, fmt.Errorf("index %q has malformed key list", name)
	}
	parts, err := splitSQLTop(rest[open+1:end], ',')
	if err != nil {
		return sqliteschema.Index{}, fmt.Errorf("index %q: %w", name, err)
	}
	idx := sqliteschema.Index{Index: ast.Index{Name: name, Unique: unique}}
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" || strings.ContainsAny(key, "()") {
			return idx, fmt.Errorf("index %q has expression or nested index key %q unsupported by the SQLite model", name, key)
		}
		words := strings.Fields(key)
		if len(words) == 0 {
			return idx, fmt.Errorf("index %q has empty key", name)
		}
		col := unquoteIdent(words[0])
		item := sqliteschema.IndexColumn{IndexColumn: ast.IndexColumn{Expression: col}}
		for i := 1; i < len(words); i++ {
			w := strings.ToUpper(words[i])
			switch w {
			case "ASC", "DESC":
				item.Order = w
			case "COLLATE":
				if i+1 >= len(words) {
					return idx, fmt.Errorf("index %q has malformed COLLATE clause", name)
				}
				i++
				item.Collation = unquoteIdent(words[i])
			default:
				return idx, fmt.Errorf("index %q key %q uses unsupported keyword %q", name, key, words[i])
			}
		}
		idx.Columns = append(idx.Columns, item)
	}
	suffix := strings.TrimSpace(rest[end+1:])
	if partial {
		keyword, predicate, ok := takeWord(suffix)
		if !ok || !strings.EqualFold(keyword, "WHERE") || strings.TrimSpace(predicate) == "" {
			return idx, fmt.Errorf("index %q is marked partial but has no supported WHERE predicate", name)
		}
		idx.Where = strings.TrimSpace(predicate)
	} else if suffix != "" {
		return idx, fmt.Errorf("index %q has unsupported trailing SQL %q", name, suffix)
	}
	return idx, nil
}

func parseColumn(source string) (sqliteschema.Column, bool, []ast.Check, error) {
	name, rest, ok := takeWord(source)
	if !ok {
		return sqliteschema.Column{}, false, nil, errors.New("missing column name")
	}
	name = unquoteIdent(name)
	column := sqliteschema.Column{Column: ast.Column{Name: name}}
	cut := len(rest)
	for _, keyword := range []string{" PRIMARY KEY", " NOT NULL", " NULL", " UNIQUE", " CHECK", " DEFAULT", " COLLATE", " REFERENCES", " GENERATED", " AS "} {
		if i := indexWord(rest, keyword); i >= 0 && i < cut {
			cut = i
		}
	}
	column.Type = strings.TrimSpace(rest[:cut])
	tail := strings.TrimSpace(rest[cut:])
	if column.Type == "" {
		return column, false, nil, errors.New("missing declared type")
	}
	autoIncrement := false
	var checks []ast.Check
	for tail != "" {
		upper := strings.ToUpper(tail)
		switch {
		case strings.HasPrefix(upper, "PRIMARY KEY"):
			column.PrimaryKey = true
			tail = strings.TrimSpace(tail[len("PRIMARY KEY"):])
			if strings.HasPrefix(strings.ToUpper(tail), "ASC") || strings.HasPrefix(strings.ToUpper(tail), "DESC") {
				return column, false, nil, errors.New("primary-key sort direction cannot be represented")
			}
		case strings.HasPrefix(upper, "AUTOINCREMENT"):
			autoIncrement = true
			tail = strings.TrimSpace(tail[len("AUTOINCREMENT"):])
		case strings.HasPrefix(upper, "NOT NULL"):
			column.NotNull = true
			tail = strings.TrimSpace(tail[len("NOT NULL"):])
		case strings.HasPrefix(upper, "NULL"):
			tail = strings.TrimSpace(tail[len("NULL"):])
		case strings.HasPrefix(upper, "UNIQUE"):
			column.Unique = true
			tail = strings.TrimSpace(tail[len("UNIQUE"):])
		case strings.HasPrefix(upper, "DEFAULT"):
			tail = strings.TrimSpace(tail[len("DEFAULT"):])
			expression, next := takeExpression(tail)
			if expression == "" {
				return column, false, nil, errors.New("empty DEFAULT expression")
			}
			column.Default = expression
			tail = next
		case strings.HasPrefix(upper, "COLLATE"):
			word, next, ok := takeWord(strings.TrimSpace(tail[len("COLLATE"):]))
			if !ok {
				return column, false, nil, errors.New("missing collation")
			}
			column.Collation = unquoteIdent(word)
			tail = next
		case strings.HasPrefix(upper, "CHECK"):
			expression, next, ok := takeParenAfter(tail[len("CHECK"):])
			if !ok {
				return column, false, nil, errors.New("malformed CHECK constraint")
			}
			checks = append(checks, ast.Check{Name: name + "_check", Expression: expression})
			tail = next
		case strings.HasPrefix(upper, "GENERATED") || strings.HasPrefix(upper, "AS "):
			if strings.HasPrefix(upper, "GENERATED") {
				tail = strings.TrimSpace(tail[len("GENERATED"):])
				if strings.HasPrefix(strings.ToUpper(tail), "ALWAYS") {
					tail = strings.TrimSpace(tail[len("ALWAYS"):])
				}
			}
			if !strings.HasPrefix(strings.ToUpper(tail), "AS") {
				return column, false, nil, errors.New("malformed generated column")
			}
			expression, next, ok := takeParenAfter(tail[len("AS"):])
			if !ok {
				return column, false, nil, errors.New("malformed generated expression")
			}
			generatedType := "virtual"
			if strings.HasPrefix(strings.ToUpper(next), "STORED") {
				generatedType = "stored"
				next = strings.TrimSpace(next[len("STORED"):])
			} else if strings.HasPrefix(strings.ToUpper(next), "VIRTUAL") {
				next = strings.TrimSpace(next[len("VIRTUAL"):])
			}
			column.Generated = &ast.Generated{As: expression, Type: generatedType}
			tail = next
		case strings.HasPrefix(upper, "REFERENCES"):
			reference, columns, next, err := parseReference(tail)
			if err != nil {
				return column, false, nil, err
			}
			if len(columns) != 1 {
				return column, false, nil, errors.New("column-level REFERENCES must name one referenced column")
			}
			reference.Column = columns[0]
			column.References = reference
			tail = next
		default:
			return column, false, nil, fmt.Errorf("unsupported column clause %q", tail)
		}
	}
	if autoIncrement && (!column.PrimaryKey || !strings.EqualFold(strings.TrimSpace(column.Type), "INTEGER")) {
		return column, false, nil, errors.New("AUTOINCREMENT requires an INTEGER PRIMARY KEY column")
	}
	return column, autoIncrement, checks, nil
}

func parseTableConstraint(t *sqliteschema.Table, source string) error {
	name := ""
	s := strings.TrimSpace(source)
	if strings.HasPrefix(strings.ToUpper(s), "CONSTRAINT ") {
		_, r, ok := takeWord(s)
		if !ok {
			return errors.New("malformed CONSTRAINT")
		}
		word, r, ok := takeWord(r)
		if !ok {
			return errors.New("missing constraint name")
		}
		name = unquoteIdent(word)
		s = strings.TrimSpace(r)
	}
	u := strings.ToUpper(s)
	if strings.HasPrefix(u, "PRIMARY KEY") {
		cols, rest, err := constraintColumns(s[len("PRIMARY KEY"):])
		if err != nil {
			return err
		}
		if rest != "" {
			return fmt.Errorf("unsupported primary-key clause %q", rest)
		}
		if name == "" {
			name = t.Name + "_pkey"
		}
		t.PrimaryKeys = append(t.PrimaryKeys, ast.PrimaryKey{Name: name, Columns: cols})
		return nil
	}
	if strings.HasPrefix(u, "UNIQUE") {
		cols, rest, err := constraintColumns(s[len("UNIQUE"):])
		if err != nil {
			return err
		}
		if rest != "" {
			return fmt.Errorf("unsupported unique-constraint clause %q", rest)
		}
		if name == "" {
			name = t.Name + "_" + strings.Join(cols, "_") + "_key"
		}
		t.UniqueConstraints = append(t.UniqueConstraints, ast.UniqueConstraint{Name: name, Columns: cols})
		return nil
	}
	if strings.HasPrefix(u, "CHECK") {
		expr, rest, ok := takeParenAfter(s[len("CHECK"):])
		if !ok || strings.TrimSpace(rest) != "" {
			return errors.New("unsupported CHECK syntax")
		}
		if name == "" {
			name = t.Name + "_check"
		}
		t.Checks = append(t.Checks, ast.Check{Name: name, Expression: expr})
		return nil
	}
	if strings.HasPrefix(u, "FOREIGN KEY") {
		columns, rest, err := constraintColumns(s[len("FOREIGN KEY"):])
		if err != nil {
			return err
		}
		ref, referencedColumns, tail, err := parseReference(rest)
		if err != nil {
			return err
		}
		deferrable, initially, err := parseForeignKeyTail(tail)
		if err != nil {
			return err
		}
		if name == "" {
			name = t.Name + "_" + strings.Join(columns, "_") + "_fkey"
		}
		t.ForeignKeys = append(t.ForeignKeys, ast.ForeignKeyConstraint{
			Name: name, Columns: columns, ReferencedTable: ref.Table, ReferencedColumns: referencedColumns,
			OnDelete: ref.OnDelete, OnUpdate: ref.OnUpdate, Deferrable: deferrable, Initially: initially,
		})
		return nil
	}
	return errors.New("unsupported table constraint")
}

func parseReference(s string) (*ast.ForeignKey, []string, string, error) {
	_, rest, ok := takeWord(s)
	if !ok {
		return nil, nil, "", errors.New("malformed REFERENCES")
	}
	table, rest, ok := takeWord(rest)
	if !ok {
		return nil, nil, "", errors.New("missing referenced table")
	}
	columns, rest, err := constraintColumns(rest)
	if err != nil {
		return nil, nil, "", fmt.Errorf("referenced columns: %w", err)
	}
	key := &ast.ForeignKey{Table: unquoteIdent(table)}
	if len(columns) == 1 {
		key.Column = columns[0]
	}
	for strings.TrimSpace(rest) != "" {
		rest = strings.TrimSpace(rest)
		upper := strings.ToUpper(rest)
		var action string
		switch {
		case strings.HasPrefix(upper, "ON DELETE "):
			action, rest, err = parseReferenceAction(rest[len("ON DELETE "):])
			key.OnDelete = action
		case strings.HasPrefix(upper, "ON UPDATE "):
			action, rest, err = parseReferenceAction(rest[len("ON UPDATE "):])
			key.OnUpdate = action
		default:
			return key, columns, rest, nil
		}
		if err != nil {
			return nil, nil, "", err
		}
	}
	return key, columns, "", nil
}

func parseReferenceAction(s string) (string, string, error) {
	first, rest, ok := takeWord(s)
	if !ok {
		return "", "", errors.New("missing foreign-key action")
	}
	action := strings.ToUpper(first)
	switch action {
	case "CASCADE", "RESTRICT":
		return action, rest, nil
	case "SET", "NO":
		second, remaining, ok := takeWord(rest)
		if !ok {
			return "", "", fmt.Errorf("incomplete foreign-key action %q", first)
		}
		second = strings.ToUpper(second)
		if action == "SET" && (second == "NULL" || second == "DEFAULT") {
			return action + " " + second, remaining, nil
		}
		if action == "NO" && second == "ACTION" {
			return action + " " + second, remaining, nil
		}
	}
	return "", "", fmt.Errorf("unsupported foreign-key action %q", first)
}

func parseForeignKeyTail(s string) (bool, string, error) {
	s = strings.TrimSpace(s)
	deferrable := false
	initially := ""
	upper := strings.ToUpper(s)
	switch {
	case strings.HasPrefix(upper, "NOT DEFERRABLE"):
		s = strings.TrimSpace(s[len("NOT DEFERRABLE"):])
	case strings.HasPrefix(upper, "DEFERRABLE"):
		deferrable = true
		s = strings.TrimSpace(s[len("DEFERRABLE"):])
	}
	if strings.HasPrefix(strings.ToUpper(s), "INITIALLY ") {
		value, rest, ok := takeWord(strings.TrimSpace(s[len("INITIALLY "):]))
		if !ok || (!strings.EqualFold(value, "DEFERRED") && !strings.EqualFold(value, "IMMEDIATE")) {
			return false, "", errors.New("unsupported foreign-key initial mode")
		}
		initially = strings.ToUpper(value)
		s = rest
	}
	if s != "" {
		return false, "", fmt.Errorf("unsupported foreign-key clause %q", s)
	}
	if initially != "" && !deferrable {
		return false, "", errors.New("INITIALLY requires a DEFERRABLE foreign key")
	}
	return deferrable, initially, nil
}

func constraintColumns(s string) ([]string, string, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '(' {
		return nil, s, errors.New("expected parenthesized columns")
	}
	end := matchingParen(s, 0)
	if end < 0 {
		return nil, s, errors.New("unclosed column list")
	}
	parts, err := splitSQLTop(s[1:end], ',')
	if err != nil {
		return nil, s, err
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		w := strings.Fields(strings.TrimSpace(p))
		if len(w) != 1 {
			return nil, s, errors.New("constraint expressions or sort options unsupported")
		}
		out = append(out, unquoteIdent(w[0]))
	}
	return out, strings.TrimSpace(s[end+1:]), nil
}

func parseTrigger(name, target, source string) (sqliteschema.Trigger, error) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(source), ";"))
	re := regexp.MustCompile(`(?is)^CREATE\s+(TEMP(?:ORARY)?\s+)?TRIGGER\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"([^"]+)"|` + "`([^`]+)`" + `|\[([^]]+)\]|([^\s]+))\s+(BEFORE|AFTER|INSTEAD\s+OF)\s+(INSERT|UPDATE(?:\s+OF\s+[^\s]+(?:\s*,\s*[^\s]+)*)?|DELETE)\s+ON\s+(?:"([^"]+)"|([^\s]+))\s+(FOR\s+EACH\s+ROW)?\s*(?:WHEN\s+(.*?))?\s*BEGIN\s+(.*)\s+END$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return sqliteschema.Trigger{}, fmt.Errorf("trigger %q on %q uses unsupported CREATE TRIGGER syntax", name, target)
	}
	sourceName := firstNonEmpty(m[2], m[3], m[4], m[5])
	if unquoteIdent(sourceName) != name {
		return sqliteschema.Trigger{}, fmt.Errorf("trigger %q catalog name does not match its SQL", name)
	}
	sourceTarget := firstNonEmpty(m[8], m[9])
	if unquoteIdent(sourceTarget) != target {
		return sqliteschema.Trigger{}, fmt.Errorf("trigger %q targets %q in SQL but catalog reports %q", name, sourceTarget, target)
	}
	event := strings.ToUpper(strings.Fields(m[7])[0])
	updates := []string{}
	if strings.HasPrefix(strings.ToUpper(m[7]), "UPDATE OF ") {
		event = "UPDATE"
		for _, x := range strings.Split(strings.TrimSpace(m[7][10:]), ",") {
			updates = append(updates, unquoteIdent(strings.TrimSpace(x)))
		}
	}
	when := strings.TrimSpace(m[11])
	body := strings.TrimSpace(m[12])
	if strings.Contains(strings.ToUpper(body), " CREATE TRIGGER ") {
		return sqliteschema.Trigger{}, fmt.Errorf("trigger %q contains unsupported nested trigger syntax", name)
	}
	return sqliteschema.Trigger{Name: name, Target: target, Timing: strings.ToUpper(m[6]), Events: []string{event}, When: when, Body: body, ForEachRow: strings.TrimSpace(m[10]) != "", UpdateOfColumns: updates}, nil
}

func createBody(source, kind, name string) (string, error) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(source), ";"))
	u := strings.ToUpper(s)
	i := strings.Index(u, "CREATE ")
	if i != 0 {
		return "", fmt.Errorf("unsupported CREATE %s statement", kind)
	}
	at := strings.Index(u, kind)
	if at < 0 {
		return "", fmt.Errorf("unsupported CREATE %s statement", kind)
	}
	rest := strings.TrimSpace(s[at+len(kind):])
	if strings.HasPrefix(strings.ToUpper(rest), "IF NOT EXISTS") {
		rest = strings.TrimSpace(rest[len("IF NOT EXISTS"):])
	}
	word, tail, ok := takeWord(rest)
	if !ok || unquoteIdent(word) != name { // indexes include their table before body
		if kind != "INDEX" {
			return "", fmt.Errorf("catalog name does not match CREATE %s name", kind)
		}
	}
	_ = tail
	if kind == "TABLE" {
		open := strings.Index(rest, "(")
		if open < 0 {
			return "", errors.New("missing table definition")
		}
		end := matchingParen(rest, open)
		if end < 0 {
			return "", errors.New("malformed table definition")
		}
		suffix := strings.TrimSpace(rest[end+1:])
		if suffix != "" && !strings.HasPrefix(strings.ToUpper(suffix), "WITHOUT ROWID") && !strings.HasPrefix(strings.ToUpper(suffix), "STRICT") {
			return "", fmt.Errorf("unsupported table option %q", suffix)
		}
		return rest[open+1 : end], nil
	}
	return rest, nil
}

func splitSQLTop(s string, sep byte) ([]string, error) {
	var out []string
	start, depth := 0, 0
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				if i+1 < len(s) && s[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '[':
			quote = ']'
		case ']':
			if quote == ']' {
				quote = 0
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, errors.New("unbalanced parentheses")
			}
		default:
			if c == sep && depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	if quote != 0 || depth != 0 {
		return nil, errors.New("unbalanced SQL quoting or parentheses")
	}
	return append(out, s[start:]), nil
}

func matchingParen(s string, start int) int {
	depth := 0
	quote := byte(0)
	for i := start; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				if i+1 < len(s) && s[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '[':
			quote = ']'
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func takeParenAfter(s string) (string, string, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '(' {
		return "", s, false
	}
	end := matchingParen(s, 0)
	if end < 0 {
		return "", s, false
	}
	return strings.TrimSpace(s[1:end]), strings.TrimSpace(s[end+1:]), true
}
func firstWord(s string) string { w, _, _ := takeWord(s); return w }
func takeWord(s string) (string, string, bool) {
	s = strings.TrimLeft(s, " \t\r\n")
	if s == "" {
		return "", "", false
	}
	start := 0
	if s[0] == '"' || s[0] == '`' || s[0] == '[' {
		end := s[0]
		if end == '[' {
			end = ']'
		}
		for i := 1; i < len(s); i++ {
			if s[i] == end {
				return s[:i+1], strings.TrimSpace(s[i+1:]), true
			}
		}
		return "", "", false
	}
	for start < len(s) && !strings.ContainsRune(" \t\r\n(),", rune(s[start])) {
		start++
	}
	return s[:start], strings.TrimSpace(s[start:]), start > 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func unquoteIdent(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '`' && s[len(s)-1] == '`') || (s[0] == '[' && s[len(s)-1] == ']') {
			s = s[1 : len(s)-1]
		}
	}
	return strings.ReplaceAll(s, "\"\"", "")
}
func quoteIdent(s string) string   { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func indexWord(s, word string) int { return strings.Index(strings.ToUpper(s), strings.ToUpper(word)) }

func takeExpression(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if s[0] == '(' {
		if end := matchingParen(s, 0); end >= 0 {
			return strings.TrimSpace(s[:end+1]), strings.TrimSpace(s[end+1:])
		}
	}
	depth := 0
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '(' {
			depth++
		}
		if c == ')' {
			depth--
		}
		if depth == 0 && (c == ' ' || c == '\t' || c == '\n') {
			rest := strings.ToUpper(strings.TrimSpace(s[i:]))
			for _, k := range []string{"NOT NULL", "NULL", "PRIMARY KEY", "UNIQUE", "CHECK", "COLLATE", "REFERENCES", "GENERATED", "AS "} {
				if strings.HasPrefix(rest, k) {
					return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i:])
				}
			}
		}
	}
	return strings.TrimSpace(s), ""
}

func parseView(name, source string) (sqliteschema.View, error) {
	body, err := createBody(source, "VIEW", name)
	if err != nil {
		return sqliteschema.View{}, fmt.Errorf("view %q: %w", name, err)
	}
	sourceName, rest, ok := takeWord(body)
	if !ok || unquoteIdent(sourceName) != name {
		return sqliteschema.View{}, fmt.Errorf("view %q catalog name does not match its SQL", name)
	}
	view := sqliteschema.View{Name: name}
	if strings.HasPrefix(strings.TrimSpace(rest), "(") {
		aliases, remaining, ok := takeParenAfter(rest)
		if !ok {
			return sqliteschema.View{}, fmt.Errorf("view %q has malformed column aliases", name)
		}
		parts, err := splitSQLTop(aliases, ',')
		if err != nil {
			return sqliteschema.View{}, fmt.Errorf("view %q aliases: %w", name, err)
		}
		for _, part := range parts {
			alias, tail, ok := takeWord(part)
			if !ok || tail != "" {
				return sqliteschema.View{}, fmt.Errorf("view %q has unsupported column alias %q", name, part)
			}
			view.ColumnAliases = append(view.ColumnAliases, unquoteIdent(alias))
		}
		rest = remaining
	}
	keyword, query, ok := takeWord(rest)
	if !ok || !strings.EqualFold(keyword, "AS") {
		return sqliteschema.View{}, fmt.Errorf("view %q has unsupported CREATE VIEW syntax", name)
	}
	view.Query = strings.TrimSpace(query)
	if view.Query == "" {
		return sqliteschema.View{}, fmt.Errorf("view %q has an empty query", name)
	}
	return view, nil
}
