package plan

import (
	"encoding/json"
	"fmt"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

type Planner struct{}

func (Planner) Dialect() string {
	return "postgresql"
}

func (Planner) PlanSnapshotDiff(previousJSON, currentJSON []byte) (*migrateplan.Plan, error) {
	return SnapshotDiff(previousJSON, currentJSON)
}

func SnapshotDiff(previousJSON, currentJSON []byte) (*migrateplan.Plan, error) {
	var previous, current pgschema.Document
	if err := json.Unmarshal(previousJSON, &previous); err != nil {
		return nil, fmt.Errorf("parse previous snapshot: %w", err)
	}
	if err := json.Unmarshal(currentJSON, &current); err != nil {
		return nil, fmt.Errorf("parse current snapshot: %w", err)
	}

	planner := planner{plan: &migrateplan.Plan{}}
	if err := planner.roles(previous.Roles, current.Roles); err != nil {
		return nil, err
	}
	if err := planner.namespaces(previous.Namespaces, current.Namespaces, occupiedSchemas(previous)); err != nil {
		return nil, err
	}
	if err := planner.extensions(previous.Extensions, current.Extensions); err != nil {
		return nil, err
	}
	if err := planner.collations(previous.Collations, current.Collations); err != nil {
		return nil, err
	}
	if err := planner.enums(previous.Enums, current.Enums); err != nil {
		return nil, err
	}
	if err := planner.compositeTypes(previous.CompositeTypes, current.CompositeTypes); err != nil {
		return nil, err
	}
	if err := planner.domains(previous.Domains, current.Domains); err != nil {
		return nil, err
	}
	if err := planner.sequences(previous.Sequences, current.Sequences); err != nil {
		return nil, err
	}
	if err := planner.functions(previous.Functions, current.Functions); err != nil {
		return nil, err
	}
	if err := planner.tables(previous.Tables, current.Tables); err != nil {
		return nil, err
	}
	if err := planner.sequenceOwnerships(previous.Sequences, current.Sequences); err != nil {
		return nil, err
	}
	if err := planner.policies(previous.Policies, current.Policies); err != nil {
		return nil, err
	}
	if err := planner.views(previous.Views, current.Views, current.Tables, current.MaterializedViews); err != nil {
		return nil, err
	}
	if err := planner.materializedViews(previous.MaterializedViews, current.MaterializedViews, current.Tables, current.Views); err != nil {
		return nil, err
	}
	if err := planner.triggers(previous.Triggers, current.Triggers, current.Tables, current.Views, current.MaterializedViews); err != nil {
		return nil, err
	}
	if err := planner.grants(previous.Grants, current.Grants); err != nil {
		return nil, err
	}
	if err := planner.rawSQL(previous.RawSQL, current.RawSQL); err != nil {
		return nil, err
	}
	if err := planner.sortChanges(); err != nil {
		return nil, err
	}
	return planner.plan, nil
}

type planner struct {
	plan *migrateplan.Plan
}

func (p planner) addWith(change migrateplan.Change) {
	p.plan.Changes = append(p.plan.Changes, change)
	for _, statement := range change.Statements {
		p.plan.Statements = append(p.plan.Statements, statement.SQL)
	}
}

func (p planner) sortChanges() error {
	changes, err := sortChangesByDependencies(p.plan.Changes)
	if err != nil {
		return err
	}
	p.plan.Changes = changes
	p.plan.Statements = p.plan.Statements[:0]
	for _, change := range changes {
		for _, statement := range change.Statements {
			p.plan.Statements = append(p.plan.Statements, statement.SQL)
		}
	}
	return nil
}

func sortChangesByDependencies(changes []migrateplan.Change) ([]migrateplan.Change, error) {
	byObject := make(map[migrateplan.ObjectRef]int, len(changes))
	for i, change := range changes {
		if _, ok := byObject[change.Object]; ok {
			continue
		}
		byObject[change.Object] = i
	}

	// A dependency edge means the referenced object must exist before this
	// change. For creates/alters that means the dependency is emitted first.
	// For drops the ordering inverts: an object must be dropped before the
	// objects it depends on, so a dropped object's dependents (other drops
	// that reference it) must be emitted first. dropDependents maps an object
	// ref to the drop changes that depend on it.
	dropDependents := make(map[migrateplan.ObjectRef][]int, len(changes))
	for i, change := range changes {
		if change.Op != migrateplan.OperationDrop {
			continue
		}
		for _, dependency := range change.Dependencies {
			dropDependents[dependency] = append(dropDependents[dependency], i)
		}
	}

	visiting := make(map[int]bool, len(changes))
	visited := make(map[int]bool, len(changes))
	sorted := make([]migrateplan.Change, 0, len(changes))

	var visit func(int) error
	visit = func(i int) error {
		if visited[i] {
			return nil
		}
		if visiting[i] {
			return unsupported("migration changes have a dependency cycle")
		}
		visiting[i] = true
		if changes[i].Op == migrateplan.OperationDrop {
			for _, j := range dropDependents[changes[i].Object] {
				if j == i {
					continue
				}
				if err := visit(j); err != nil {
					return err
				}
			}
		} else {
			for _, dependency := range changes[i].Dependencies {
				j, ok := byObject[dependency]
				if !ok || j == i {
					continue
				}
				if changes[j].Op == migrateplan.OperationDrop {
					continue
				}
				if err := visit(j); err != nil {
					return err
				}
			}
		}
		visiting[i] = false
		visited[i] = true
		sorted = append(sorted, changes[i])
		return nil
	}

	for i := range changes {
		if err := visit(i); err != nil {
			return nil, err
		}
	}
	return sorted, nil
}
