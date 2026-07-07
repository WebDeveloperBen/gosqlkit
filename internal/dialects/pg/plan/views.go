package plan

import (
	"reflect"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func (p planner) views(previous, current []pgschema.View, tables []ast.Table, materializedViews []pgschema.MaterializedView) error {
	prev := mapBy(previous, viewKey)
	for _, view := range sortedBy(current, viewKey) {
		key := viewKey(view)
		old, ok := prev[key]
		if !ok {
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindView, key),
					"create view "+key,
					migrateplan.SQL(renderView(view)),
				).WithDependencies(
					dependencyRefs(view.DependsOn, tables, current, materializedViews, migrateplan.Ref(migrateplan.ObjectKindView, key))...,
				).WithReverse(
					migrateplan.SQL("DROP VIEW " + renderQualified(view.Schema, view.Name) + ";"),
				),
			)
			if view.Comment != "" {
				p.addWith(viewCommentChange(view))
			}
			continue
		}
		oldWithoutComment, currentWithoutComment := old, view
		oldWithoutComment.Comment = ""
		currentWithoutComment.Comment = ""
		if !reflect.DeepEqual(oldWithoutComment, currentWithoutComment) {
			return unsupported("view modifications require semantic planning")
		}
		if old.Comment == "" && view.Comment != "" {
			p.addWith(viewCommentChange(view))
		} else if old.Comment != view.Comment {
			if view.Comment == "" {
				return unsupportedDestructive("view comments were removed")
			}
			return unsupported("view comment modifications require semantic planning")
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("views were removed")
	}
	return nil
}

func (p planner) materializedViews(previous, current []pgschema.MaterializedView, tables []ast.Table, views []pgschema.View) error {
	prev := mapBy(previous, materializedViewKey)
	for _, view := range sortedBy(current, materializedViewKey) {
		key := materializedViewKey(view)
		old, ok := prev[key]
		if !ok {
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindMaterializedView, key),
					"create materialized view "+key,
					migrateplan.SQL(renderMaterializedView(view)),
				).WithDependencies(
					dependencyRefs(view.DependsOn, tables, views, current, migrateplan.Ref(migrateplan.ObjectKindMaterializedView, key))...,
				).WithReverse(
					migrateplan.SQL("DROP MATERIALIZED VIEW " + renderQualified(view.Schema, view.Name) + ";"),
				),
			)
			if view.Comment != "" {
				p.addWith(materializedViewCommentChange(view))
			}
			continue
		}
		oldWithoutComment, currentWithoutComment := old, view
		oldWithoutComment.Comment = ""
		currentWithoutComment.Comment = ""
		if !reflect.DeepEqual(oldWithoutComment, currentWithoutComment) {
			return unsupported("materialized view modifications require semantic planning")
		}
		if old.Comment == "" && view.Comment != "" {
			p.addWith(materializedViewCommentChange(view))
		} else if old.Comment != view.Comment {
			if view.Comment == "" {
				return unsupportedDestructive("materialized view comments were removed")
			}
			return unsupported("materialized view comment modifications require semantic planning")
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("materialized views were removed")
	}
	return nil
}

func viewCommentChange(view pgschema.View) migrateplan.Change {
	key := viewKey(view)
	return migrateplan.NewChange(
		migrateplan.OperationAlter,
		migrateplan.Ref(migrateplan.ObjectKindView, key),
		"comment on view "+key,
		migrateplan.SQL("COMMENT ON VIEW "+renderQualified(view.Schema, view.Name)+" IS "+quoteSQL(view.Comment)+";"),
	).WithDependencies(
		migrateplan.Ref(migrateplan.ObjectKindView, key),
	).WithReverse(
		migrateplan.SQL("COMMENT ON VIEW " + renderQualified(view.Schema, view.Name) + " IS NULL;"),
	)
}

func materializedViewCommentChange(view pgschema.MaterializedView) migrateplan.Change {
	key := materializedViewKey(view)
	return migrateplan.NewChange(
		migrateplan.OperationAlter,
		migrateplan.Ref(migrateplan.ObjectKindMaterializedView, key),
		"comment on materialized view "+key,
		migrateplan.SQL("COMMENT ON MATERIALIZED VIEW "+renderQualified(view.Schema, view.Name)+" IS "+quoteSQL(view.Comment)+";"),
	).WithDependencies(
		migrateplan.Ref(migrateplan.ObjectKindMaterializedView, key),
	).WithReverse(
		migrateplan.SQL("COMMENT ON MATERIALIZED VIEW " + renderQualified(view.Schema, view.Name) + " IS NULL;"),
	)
}

func dependencyRefs(dependsOn []string, tables []ast.Table, views []pgschema.View, materializedViews []pgschema.MaterializedView, self migrateplan.ObjectRef) []migrateplan.ObjectRef {
	refs := make([]migrateplan.ObjectRef, 0, len(dependsOn))
	for _, dependency := range dependsOn {
		ref := dependencyRef(dependency, tables, views, materializedViews)
		if ref != self {
			refs = append(refs, ref)
		}
	}
	return uniqueRefs(refs)
}

func dependencyRef(dependency string, tables []ast.Table, views []pgschema.View, materializedViews []pgschema.MaterializedView) migrateplan.ObjectRef {
	key := referencedTableKey(dependency)
	for _, table := range tables {
		if tableKey(table) == key {
			return migrateplan.Ref(migrateplan.ObjectKindTable, key)
		}
	}
	for _, view := range views {
		if viewKey(view) == key {
			return migrateplan.Ref(migrateplan.ObjectKindView, key)
		}
	}
	for _, materializedView := range materializedViews {
		if materializedViewKey(materializedView) == key {
			return migrateplan.Ref(migrateplan.ObjectKindMaterializedView, key)
		}
	}
	return migrateplan.Ref(migrateplan.ObjectKindTable, key)
}
