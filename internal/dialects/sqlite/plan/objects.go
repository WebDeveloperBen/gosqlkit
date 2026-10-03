package plan

import (
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/render"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func (p planner) rawSQL(previous, current []sqliteschema.RawSQL) error {
	prev := migrateplan.MapBy(previous, rawSQLName)
	for _, block := range migrateplan.SortedBy(current, rawSQLName) {
		old, ok := prev[block.Name]
		if !ok {
			change := migrateplan.NewChange(
				migrateplan.OperationCreate,
				migrateplan.Ref(migrateplan.ObjectKindRawSQL, block.Name),
				"apply raw SQL block "+block.Name,
				migrateplan.SQL(strings.TrimSpace(block.SQL)),
			).WithRisks(migrateplan.RiskRequiresDDLReview)
			if down := strings.TrimSpace(block.Down); down != "" {
				change = change.WithReverse(migrateplan.SQL(down))
			}
			p.addWith(change)
			continue
		}
		if strings.TrimSpace(old.SQL) != strings.TrimSpace(block.SQL) || old.Before != block.Before {
			return unsupported("raw SQL block " + block.Name + " modification requires manual migration authoring; gosqlkit cannot infer how to migrate arbitrary SQL")
		}
		delete(prev, block.Name)
	}
	if len(prev) > 0 {
		return unsupported("raw SQL block " + mapKeys(prev)[0] + " removal requires manual migration authoring; gosqlkit cannot infer how to reverse arbitrary SQL")
	}
	return nil
}

func (p planner) views(previous, current []sqliteschema.View) error {
	prev := migrateplan.MapBy(previous, viewName)
	for _, view := range migrateplan.SortedBy(current, viewName) {
		old, ok := prev[view.Name]
		if !ok {
			if view.PreviousName != "" {
				if oldView, has := prev[view.PreviousName]; has {
					// SQLite has no ALTER VIEW RENAME; drop and recreate.
					p.replaceView(oldView, view, fmt.Sprintf("rename view %s to %s", oldView.Name, view.Name))
					delete(prev, view.PreviousName)
					continue
				}
				return unsupported(fmt.Sprintf("view %q declares previousName %q but no such view exists in the previous snapshot", view.Name, view.PreviousName))
			}
			p.createView(view)
			continue
		}
		if !jsonEqual(viewComparable(old), viewComparable(view)) {
			p.replaceView(old, view, "redefine view "+view.Name)
		}
		delete(prev, view.Name)
	}
	for _, name := range mapKeys(prev) {
		p.dropView(prev[name])
	}
	return nil
}

func (p planner) createView(view sqliteschema.View) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationCreate,
		migrateplan.Ref(migrateplan.ObjectKindView, view.Name),
		"create view "+view.Name,
		migrateplan.SQL(render.CreateView(view)),
	).WithReverse(migrateplan.SQL("DROP VIEW " + view.Name + ";")))
}

func (p planner) dropView(view sqliteschema.View) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationDrop,
		migrateplan.Ref(migrateplan.ObjectKindView, view.Name),
		"drop view "+view.Name,
		migrateplan.SQL("DROP VIEW "+view.Name+";"),
	).WithRisks(migrateplan.RiskDestructive).
		WithReverse(migrateplan.SQL(render.CreateView(view))))
}

func (p planner) replaceView(old, view sqliteschema.View, summary string) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationReplace,
		migrateplan.Ref(migrateplan.ObjectKindView, view.Name),
		summary,
		migrateplan.SQL("DROP VIEW "+old.Name+";"),
		migrateplan.SQL(render.CreateView(view)),
	).WithReverse(
		migrateplan.SQL("DROP VIEW "+view.Name+";"),
		migrateplan.SQL(render.CreateView(old)),
	))
}

func (p planner) triggers(previous, current []sqliteschema.Trigger) error {
	prev := migrateplan.MapBy(previous, triggerName)
	for _, trigger := range migrateplan.SortedBy(current, triggerName) {
		// Triggers on a rebuilt table are dropped and recreated by the rebuild
		// change itself, so skip them here to avoid duplicate or conflicting DDL.
		if p.rebuiltTables[trigger.Target] {
			delete(prev, trigger.Name)
			continue
		}
		old, ok := prev[trigger.Name]
		if !ok {
			if trigger.PreviousName != "" {
				if oldTrigger, has := prev[trigger.PreviousName]; has {
					p.replaceTrigger(oldTrigger, trigger, fmt.Sprintf("rename trigger %s to %s", oldTrigger.Name, trigger.Name))
					delete(prev, trigger.PreviousName)
					continue
				}
				return unsupported(fmt.Sprintf("trigger %q declares previousName %q but no such trigger exists in the previous snapshot", trigger.Name, trigger.PreviousName))
			}
			p.createTrigger(trigger)
			continue
		}
		if !jsonEqual(triggerComparable(old), triggerComparable(trigger)) {
			p.replaceTrigger(old, trigger, "redefine trigger "+trigger.Name)
		}
		delete(prev, trigger.Name)
	}
	for _, name := range mapKeys(prev) {
		trigger := prev[name]
		if p.rebuiltTables[trigger.Target] {
			continue // dropped together with the rebuilt table
		}
		p.dropTrigger(trigger)
	}
	return nil
}

func (p planner) createTrigger(trigger sqliteschema.Trigger) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationCreate,
		migrateplan.Ref(migrateplan.ObjectKindTrigger, trigger.Name),
		"create trigger "+trigger.Name,
		migrateplan.SQL(render.CreateTrigger(trigger)),
	).WithDependencies(migrateplan.Ref(migrateplan.ObjectKindTable, trigger.Target)).
		WithReverse(migrateplan.SQL("DROP TRIGGER " + trigger.Name + ";")))
}

func (p planner) dropTrigger(trigger sqliteschema.Trigger) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationDrop,
		migrateplan.Ref(migrateplan.ObjectKindTrigger, trigger.Name),
		"drop trigger "+trigger.Name,
		migrateplan.SQL("DROP TRIGGER "+trigger.Name+";"),
	).WithRisks(migrateplan.RiskDestructive).
		WithReverse(migrateplan.SQL(render.CreateTrigger(trigger))))
}

func (p planner) replaceTrigger(old, trigger sqliteschema.Trigger, summary string) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationReplace,
		migrateplan.Ref(migrateplan.ObjectKindTrigger, trigger.Name),
		summary,
		migrateplan.SQL("DROP TRIGGER "+old.Name+";"),
		migrateplan.SQL(render.CreateTrigger(trigger)),
	).WithDependencies(migrateplan.Ref(migrateplan.ObjectKindTable, trigger.Target)).
		WithReverse(
			migrateplan.SQL("DROP TRIGGER "+trigger.Name+";"),
			migrateplan.SQL(render.CreateTrigger(old)),
		))
}

func (p planner) createIndex(tableName string, index sqliteschema.Index) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationCreate,
		migrateplan.Ref(migrateplan.ObjectKindIndex, index.Name),
		"create index "+index.Name,
		migrateplan.SQL(render.CreateIndex(tableName, index)),
	).WithDependencies(migrateplan.Ref(migrateplan.ObjectKindTable, tableName)).
		WithReverse(migrateplan.SQL("DROP INDEX " + index.Name + ";")))
}

func (p planner) dropIndex(tableName string, index sqliteschema.Index) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationDrop,
		migrateplan.Ref(migrateplan.ObjectKindIndex, index.Name),
		"drop index "+index.Name,
		migrateplan.SQL("DROP INDEX "+index.Name+";"),
	).WithReverse(migrateplan.SQL(render.CreateIndex(tableName, index))))
}

func (p planner) renameIndex(tableName string, old, index sqliteschema.Index) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationReplace,
		migrateplan.Ref(migrateplan.ObjectKindIndex, index.Name),
		fmt.Sprintf("rename index %s to %s", old.Name, index.Name),
		migrateplan.SQL("DROP INDEX "+old.Name+";"),
		migrateplan.SQL(render.CreateIndex(tableName, index)),
	).WithDependencies(migrateplan.Ref(migrateplan.ObjectKindTable, tableName)).
		WithReverse(
			migrateplan.SQL("DROP INDEX "+index.Name+";"),
			migrateplan.SQL(render.CreateIndex(tableName, old)),
		))
}

// viewComparable strips rename metadata so a pure rename is not also seen as a
// definition change.
func viewComparable(view sqliteschema.View) sqliteschema.View {
	view.Name = ""
	view.PreviousName = ""
	view.IfNotExists = false
	return view
}

func triggerComparable(trigger sqliteschema.Trigger) sqliteschema.Trigger {
	trigger.Name = ""
	trigger.PreviousName = ""
	return trigger
}
