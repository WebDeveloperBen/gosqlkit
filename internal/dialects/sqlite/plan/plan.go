package plan

import (
	"encoding/json"
	"fmt"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

// Planner implements migrateplan.SnapshotPlanner for SQLite.
type Planner struct{}

func (Planner) Dialect() string {
	return "sqlite"
}

func (Planner) PlanSnapshotDiff(previousJSON, currentJSON []byte) (*migrateplan.Plan, error) {
	return SnapshotDiff(previousJSON, currentJSON)
}

// SnapshotDiff diffs two SQLite snapshot documents into a migration plan.
func SnapshotDiff(previousJSON, currentJSON []byte) (*migrateplan.Plan, error) {
	previous, err := decodeDocument(previousJSON, "previous")
	if err != nil {
		return nil, err
	}
	current, err := decodeDocument(currentJSON, "current")
	if err != nil {
		return nil, err
	}

	p := planner{
		plan:             &migrateplan.Plan{},
		previousTriggers: previous.Triggers,
		currentTriggers:  current.Triggers,
		rebuiltTables:    map[string]bool{},
	}
	if err := p.rawSQL(previous.RawSQL, current.RawSQL); err != nil {
		return nil, err
	}
	if err := p.tables(previous.Tables, current.Tables); err != nil {
		return nil, err
	}
	if err := p.views(previous.Views, current.Views); err != nil {
		return nil, err
	}
	if err := p.triggers(previous.Triggers, current.Triggers); err != nil {
		return nil, err
	}
	if err := p.sortChanges(); err != nil {
		return nil, err
	}
	return p.plan, nil
}

func decodeDocument(data []byte, label string) (sqliteschema.Document, error) {
	var doc sqliteschema.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return sqliteschema.Document{}, fmt.Errorf("parse %s snapshot: %w", label, err)
	}
	normalised, err := sqliteschema.NormaliseDocumentForDiff(doc)
	if err != nil {
		return sqliteschema.Document{}, fmt.Errorf("normalise %s snapshot: %w", label, err)
	}
	return normalised, nil
}

type planner struct {
	plan             *migrateplan.Plan
	previousTriggers []sqliteschema.Trigger
	currentTriggers  []sqliteschema.Trigger
	rebuiltTables    map[string]bool
}

func (p planner) addWith(change migrateplan.Change) {
	p.plan.Changes = append(p.plan.Changes, change)
	for _, statement := range change.Statements {
		p.plan.Statements = append(p.plan.Statements, statement.SQL)
	}
}

func (p planner) sortChanges() error {
	changes, err := migrateplan.SortChangesByDependencies(p.plan.Changes)
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
