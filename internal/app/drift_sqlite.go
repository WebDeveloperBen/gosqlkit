package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

func projectDatabaseSnapshot(snapshot, dialect string) (string, error) {
	if dialect == "sqlite" {
		var document sqliteschema.Document
		if err := json.Unmarshal([]byte(snapshot), &document); err != nil {
			return "", fmt.Errorf("parse SQLite snapshot for drift projection: %w", err)
		}
		raw, err := sqliteschema.JSON("sqlite", projectSQLiteDriftSchema(sqliteschema.Schema{
			Tables:   document.Tables,
			Views:    document.Views,
			Triggers: document.Triggers,
		}))
		if err != nil {
			return "", err
		}
		return injectSnapshotIDs(string(raw), "")
	}
	var document pgschema.Document
	if err := json.Unmarshal([]byte(snapshot), &document); err != nil {
		return "", fmt.Errorf("parse PostgreSQL snapshot for drift projection: %w", err)
	}
	raw, err := pgschema.JSON("postgresql", projectDriftDocument(document))
	if err != nil {
		return "", err
	}
	return injectSnapshotIDs(string(raw), "")
}

func projectSQLiteDriftSchema(schema sqliteschema.Schema) sqliteschema.Schema {
	tables := make([]sqliteschema.Table, 0, len(schema.Tables))
	for _, table := range schema.Tables {
		if table.Name == "goose_db_version" || table.Name == "schema_migrations" {
			continue
		}
		table.PreviousName = ""
		table.Comment = ""
		table.IfNotExists = false
		table.Columns = append([]sqliteschema.Column(nil), table.Columns...)
		integerPrimaryKeyTypes := make(map[string]bool, len(table.Columns))
		for _, column := range table.Columns {
			integerPrimaryKeyTypes[column.Name] = strings.EqualFold(strings.TrimSpace(column.Type), "INTEGER")
		}
		for i := range table.Columns {
			column := &table.Columns[i]
			column.PreviousName = ""
			column.Comment = ""
			column.Type = sqliteTypeAffinity(column.Type)
			column.Default = normalizeSQLiteSQL(column.Default)
			column.Collation = strings.ToUpper(strings.TrimSpace(column.Collation))
			if column.Generated != nil {
				generated := *column.Generated
				generated.As = normalizeSQLiteSQL(generated.As)
				generated.Type = strings.ToLower(strings.TrimSpace(generated.Type))
				column.Generated = &generated
			}
			if column.References != nil {
				reference := *column.References
				reference.OnDelete = normalizeSQLiteSQL(strings.ToUpper(reference.OnDelete))
				reference.OnUpdate = normalizeSQLiteSQL(strings.ToUpper(reference.OnUpdate))
				column.References = &reference
			}
		}
		table.PrimaryKeys = append(table.PrimaryKeys[:0:0], table.PrimaryKeys...)
		for i := range table.PrimaryKeys {
			table.PrimaryKeys[i].PreviousName = ""
		}
		if !table.WithoutRowID {
			var primaryColumns []string
			for _, key := range table.PrimaryKeys {
				primaryColumns = append(primaryColumns, key.Columns...)
			}
			for _, column := range table.Columns {
				if column.PrimaryKey {
					primaryColumns = append(primaryColumns, column.Name)
				}
			}
			if len(primaryColumns) == 1 {
				for i := range table.Columns {
					column := &table.Columns[i]
					if column.Name == primaryColumns[0] && integerPrimaryKeyTypes[column.Name] {
						column.PrimaryKey = true
						table.PrimaryKeys = nil
						break
					}
				}
			}
		}
		table.UniqueConstraints = append(table.UniqueConstraints[:0:0], table.UniqueConstraints...)
		for i := range table.UniqueConstraints {
			table.UniqueConstraints[i].PreviousName = ""
		}
		table.ForeignKeys = append(table.ForeignKeys[:0:0], table.ForeignKeys...)
		for i := range table.ForeignKeys {
			key := &table.ForeignKeys[i]
			key.PreviousName = ""
			key.OnDelete = normalizeSQLiteSQL(strings.ToUpper(key.OnDelete))
			key.OnUpdate = normalizeSQLiteSQL(strings.ToUpper(key.OnUpdate))
			key.Initially = strings.ToUpper(strings.TrimSpace(key.Initially))
		}
		table.Checks = append(table.Checks[:0:0], table.Checks...)
		for i := range table.Checks {
			table.Checks[i].PreviousName = ""
			table.Checks[i].Expression = normalizeSQLiteSQL(table.Checks[i].Expression)
		}
		table.Indexes = append([]sqliteschema.Index(nil), table.Indexes...)
		for i := range table.Indexes {
			index := &table.Indexes[i]
			index.PreviousName = ""
			index.IfNotExists = false
			index.Where = normalizeSQLiteSQL(index.Where)
			for j := range index.Columns {
				index.Columns[j].Expression = normalizeSQLiteSQL(index.Columns[j].Expression)
				index.Columns[j].Order = strings.ToUpper(strings.TrimSpace(index.Columns[j].Order))
				index.Columns[j].Collation = strings.ToUpper(strings.TrimSpace(index.Columns[j].Collation))
			}
		}
		tables = append(tables, table)
	}
	views := make([]sqliteschema.View, 0, len(schema.Views))
	for _, view := range schema.Views {
		if view.Temporary {
			continue
		}
		view.PreviousName = ""
		view.Comment = ""
		view.DependsOn = nil
		view.IfNotExists = false
		view.Query = normalizeSQLiteSQL(view.Query)
		views = append(views, view)
	}
	triggers := append([]sqliteschema.Trigger(nil), schema.Triggers...)
	for i := range triggers {
		triggers[i].PreviousName = ""
		triggers[i].Comment = ""
		triggers[i].Timing = strings.ToUpper(strings.TrimSpace(triggers[i].Timing))
		triggers[i].Events = append([]string(nil), triggers[i].Events...)
		for j := range triggers[i].Events {
			triggers[i].Events[j] = strings.ToUpper(strings.TrimSpace(triggers[i].Events[j]))
		}
		triggers[i].When = normalizeSQLiteSQL(triggers[i].When)
		triggers[i].ForEachRow = true
		triggers[i].Body = normalizeSQLiteSQL(triggers[i].Body)
	}
	return sqliteschema.Schema{Tables: tables, Views: views, Triggers: triggers}
}

func sqliteTypeAffinity(declaredType string) string {
	value := strings.ToUpper(strings.TrimSpace(declaredType))
	switch {
	case strings.Contains(value, "INT"):
		return "integer"
	case strings.Contains(value, "CHAR"), strings.Contains(value, "CLOB"), strings.Contains(value, "TEXT"):
		return "text"
	case value == "", strings.Contains(value, "BLOB"):
		return "blob"
	case strings.Contains(value, "REAL"), strings.Contains(value, "FLOA"), strings.Contains(value, "DO"+"UB"):
		return "real"
	default:
		return "numeric"
	}
}

func normalizeSQLiteSQL(value string) string {
	var normalized strings.Builder
	var quote byte
	var previous byte
	pendingSpace := false
	for i := 0; i < len(value); i++ {
		character := value[i]
		if quote != 0 {
			normalized.WriteByte(character)
			previous = character
			if character == quote {
				if i+1 < len(value) && value[i+1] == quote && quote != ']' {
					i++
					normalized.WriteByte(value[i])
					previous = value[i]
				} else {
					quote = 0
				}
			}
			continue
		}
		if character == ' ' || character == '\t' || character == '\n' || character == '\r' || character == '\f' {
			pendingSpace = true
			continue
		}
		if pendingSpace && normalized.Len() > 0 && !sqlitePunctuation(character) && !sqlitePunctuation(previous) {
			normalized.WriteByte(' ')
		}
		pendingSpace = false
		switch character {
		case '\'', '"', '`':
			quote = character
		case '[':
			quote = ']'
		default:
			if character >= 'A' && character <= 'Z' {
				character += 'a' - 'A'
			}
		}
		normalized.WriteByte(character)
		previous = character
	}
	return strings.TrimSpace(normalized.String())
}

func sqlitePunctuation(character byte) bool {
	return character == '(' || character == ')' || character == ',' || character == '.'
}

func sqliteDriftDifferences(desired, database sqliteschema.Schema) []DriftDifference {
	var diffs []DriftDifference
	compareDriftCollection(&diffs, "table", sqliteShallowTables(desired.Tables), sqliteShallowTables(database.Tables), func(item sqliteschema.Table) string {
		return item.Name
	})
	compareDriftCollection(&diffs, "view", desired.Views, database.Views, func(item sqliteschema.View) string {
		return item.Name
	})
	compareDriftCollection(&diffs, "trigger", desired.Triggers, database.Triggers, func(item sqliteschema.Trigger) string {
		return item.Target + "." + item.Name
	})
	desiredTables := driftMapBy(desired.Tables, func(item sqliteschema.Table) string { return item.Name })
	databaseTables := driftMapBy(database.Tables, func(item sqliteschema.Table) string { return item.Name })
	tableNames := make([]string, 0, len(desiredTables))
	for name := range desiredTables {
		if _, ok := databaseTables[name]; ok {
			tableNames = append(tableNames, name)
		}
	}
	sort.Strings(tableNames)
	for _, name := range tableNames {
		desiredTable := desiredTables[name]
		databaseTable := databaseTables[name]
		compareDriftCollection(&diffs, "column", desiredTable.Columns, databaseTable.Columns, func(item sqliteschema.Column) string {
			return name + "." + item.Name
		})
		compareDriftCollection(&diffs, "primaryKey", desiredTable.PrimaryKeys, databaseTable.PrimaryKeys, func(item ast.PrimaryKey) string {
			return name + "." + item.Name
		})
		compareDriftCollection(&diffs, "uniqueConstraint", desiredTable.UniqueConstraints, databaseTable.UniqueConstraints, func(item ast.UniqueConstraint) string {
			return name + "." + item.Name
		})
		compareDriftCollection(&diffs, "foreignKey", desiredTable.ForeignKeys, databaseTable.ForeignKeys, func(item ast.ForeignKeyConstraint) string {
			return name + "." + item.Name
		})
		compareDriftCollection(&diffs, "check", desiredTable.Checks, databaseTable.Checks, func(item ast.Check) string {
			return name + "." + item.Name
		})
		compareDriftCollection(&diffs, "index", desiredTable.Indexes, databaseTable.Indexes, func(item sqliteschema.Index) string {
			return name + "." + item.Name
		})
	}
	return diffs
}

func sqliteShallowTables(tables []sqliteschema.Table) []sqliteschema.Table {
	out := append([]sqliteschema.Table(nil), tables...)
	for i := range out {
		out[i].Columns = nil
		out[i].Indexes = nil
		out[i].Table.Columns = nil
		out[i].PrimaryKeys = nil
		out[i].UniqueConstraints = nil
		out[i].ForeignKeys = nil
		out[i].Checks = nil
		out[i].Table.Indexes = nil
		out[i].Exclusions = nil
	}
	return out
}

func isSQLiteDialect(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sqlite", "sqlite3":
		return true
	default:
		return false
	}
}

func databaseSnapshotDifferences(desired, database, dialect string) ([]DriftDifference, error) {
	if dialect == "sqlite" {
		var desiredDocument sqliteschema.Document
		if err := json.Unmarshal([]byte(desired), &desiredDocument); err != nil {
			return nil, fmt.Errorf("parse desired SQLite drift snapshot: %w", err)
		}
		var databaseDocument sqliteschema.Document
		if err := json.Unmarshal([]byte(database), &databaseDocument); err != nil {
			return nil, fmt.Errorf("parse database SQLite drift snapshot: %w", err)
		}
		desiredSchema := projectSQLiteDriftSchema(sqliteschema.Schema{
			Tables: desiredDocument.Tables, Views: desiredDocument.Views, Triggers: desiredDocument.Triggers,
		})
		databaseSchema := projectSQLiteDriftSchema(sqliteschema.Schema{
			Tables: databaseDocument.Tables, Views: databaseDocument.Views, Triggers: databaseDocument.Triggers,
		})
		return sqliteDriftDifferences(desiredSchema, databaseSchema), nil
	}
	var desiredDocument pgschema.Document
	if err := json.Unmarshal([]byte(desired), &desiredDocument); err != nil {
		return nil, fmt.Errorf("parse desired PostgreSQL drift snapshot: %w", err)
	}
	var databaseDocument pgschema.Document
	if err := json.Unmarshal([]byte(database), &databaseDocument); err != nil {
		return nil, fmt.Errorf("parse database PostgreSQL drift snapshot: %w", err)
	}
	return driftDifferences(projectDriftDocument(desiredDocument), projectDriftDocument(databaseDocument)), nil
}
