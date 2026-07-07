package plan

import (
	"reflect"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func (p planner) triggers(previous, current []pgschema.Trigger, tables []ast.Table, views []pgschema.View, materializedViews []pgschema.MaterializedView) error {
	prev := mapBy(previous, triggerKey)
	for _, trigger := range sortedBy(current, triggerKey) {
		key := triggerKey(trigger)
		old, ok := prev[key]
		if !ok {
			if trigger.PreviousName != "" {
				return unsupported("trigger " + key + " rename metadata requires manual review; PostgreSQL does not support ALTER TRIGGER ... RENAME TO; drop and recreate the trigger manually")
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindTrigger, key),
					"create trigger "+key,
					migrateplan.SQL(renderTrigger(trigger)),
				).WithDependencies(
					triggerTargetRef(trigger, tables, views, materializedViews),
					migrateplan.Ref(migrateplan.ObjectKindFunction, triggerFunctionKey(trigger)),
				).WithReverse(
					migrateplan.SQL("DROP TRIGGER " + trigger.Name + " ON " + renderReferencedTable(trigger.Target) + ";"),
				),
			)
			if trigger.Comment != "" {
				p.addWith(triggerCommentChange(trigger, tables, views, materializedViews))
			}
			continue
		}
		oldWithoutComment, currentWithoutComment := old, trigger
		oldWithoutComment.Comment = ""
		currentWithoutComment.Comment = ""
		if !reflect.DeepEqual(oldWithoutComment, currentWithoutComment) {
			return unsupported("trigger modifications require semantic planning")
		}
		if old.Comment == "" && trigger.Comment != "" {
			p.addWith(triggerCommentChange(trigger, tables, views, materializedViews))
		} else if old.Comment != trigger.Comment {
			if trigger.Comment == "" {
				p.addWith(
					migrateplan.NewChange(
						migrateplan.OperationAlter,
						migrateplan.Ref(migrateplan.ObjectKindTrigger, key),
						"drop comment from trigger "+key,
						migrateplan.SQL("COMMENT ON TRIGGER "+trigger.Name+" ON "+renderReferencedTable(trigger.Target)+" IS NULL;"),
					).WithDependencies(
						triggerTargetRef(trigger, tables, views, materializedViews),
						migrateplan.Ref(migrateplan.ObjectKindTrigger, key),
					).WithRisks(
						migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
					).WithReverse(
						migrateplan.SQL("COMMENT ON TRIGGER " + trigger.Name + " ON " + renderReferencedTable(trigger.Target) + " IS " + quoteSQL(old.Comment) + ";"),
					),
				)
			} else {
				return unsupported("trigger comment modifications require semantic planning")
			}
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		for _, key := range sortedStrings(removedNames(prev)) {
			trigger := prev[key]
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindTrigger, key),
					"drop trigger "+key,
					migrateplan.SQL("DROP TRIGGER "+trigger.Name+" ON "+renderReferencedTable(trigger.Target)+";"),
				).WithDependencies(
					triggerTargetRef(trigger, tables, views, materializedViews),
					migrateplan.Ref(migrateplan.ObjectKindFunction, triggerFunctionKey(trigger)),
				).WithRisks(
					migrateplan.RiskDestructive,
				),
			)
		}
	}
	return nil
}

func triggerCommentChange(trigger pgschema.Trigger, tables []ast.Table, views []pgschema.View, materializedViews []pgschema.MaterializedView) migrateplan.Change {
	key := triggerKey(trigger)
	return migrateplan.NewChange(
		migrateplan.OperationAlter,
		migrateplan.Ref(migrateplan.ObjectKindTrigger, key),
		"comment on trigger "+key,
		migrateplan.SQL("COMMENT ON TRIGGER "+trigger.Name+" ON "+renderReferencedTable(trigger.Target)+" IS "+quoteSQL(trigger.Comment)+";"),
	).WithDependencies(
		triggerTargetRef(trigger, tables, views, materializedViews),
		migrateplan.Ref(migrateplan.ObjectKindTrigger, key),
	).WithReverse(
		migrateplan.SQL("COMMENT ON TRIGGER " + trigger.Name + " ON " + renderReferencedTable(trigger.Target) + " IS NULL;"),
	)
}

func triggerTargetRef(trigger pgschema.Trigger, tables []ast.Table, views []pgschema.View, materializedViews []pgschema.MaterializedView) migrateplan.ObjectRef {
	key := referencedTableKey(trigger.Target)
	for _, table := range tables {
		if tableKey(table) == key {
			return migrateplan.Ref(migrateplan.ObjectKindTable, key)
		}
	}
	for _, view := range views {
		if qualified(view.Schema, view.Name) == key {
			return migrateplan.Ref(migrateplan.ObjectKindView, key)
		}
	}
	for _, materializedView := range materializedViews {
		if qualified(materializedView.Schema, materializedView.Name) == key {
			return migrateplan.Ref(migrateplan.ObjectKindMaterializedView, key)
		}
	}
	return migrateplan.Ref(migrateplan.ObjectKindTable, key)
}
