package plan

import (
	"fmt"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func validatePreviousNames(previous, current sqliteschema.Document) error {
	if err := validateAnnotatedNames(
		"table", previous.Tables, current.Tables,
		func(table sqliteschema.Table) string { return table.Name },
		func(table sqliteschema.Table) string { return table.PreviousName },
	); err != nil {
		return err
	}

	previousTables := migrateplan.MapBy(previous.Tables, func(table sqliteschema.Table) string { return table.Name })
	for _, currentTable := range current.Tables {
		previousTableName := currentTable.Name
		if currentTable.PreviousName != "" {
			previousTableName = currentTable.PreviousName
		}
		previousTable := previousTables[previousTableName]
		if err := validateAnnotatedNames(
			"column in table "+currentTable.Name, previousTable.Columns, currentTable.Columns,
			func(column sqliteschema.Column) string { return column.Name },
			func(column sqliteschema.Column) string { return column.PreviousName },
		); err != nil {
			return err
		}
		if err := validateAnnotatedNames(
			"index in table "+currentTable.Name, previousTable.Indexes, currentTable.Indexes,
			func(index sqliteschema.Index) string { return index.Name },
			func(index sqliteschema.Index) string { return index.PreviousName },
		); err != nil {
			return err
		}
	}
	if err := validateAnnotatedNames(
		"view", previous.Views, current.Views,
		func(view sqliteschema.View) string { return view.Name },
		func(view sqliteschema.View) string { return view.PreviousName },
	); err != nil {
		return err
	}
	return validateAnnotatedNames(
		"trigger", previous.Triggers, current.Triggers,
		func(trigger sqliteschema.Trigger) string { return trigger.Name },
		func(trigger sqliteschema.Trigger) string { return trigger.PreviousName },
	)
}

func validateAnnotatedNames[T any](kind string, previous, current []T, name, previousName func(T) string) error {
	previousCounts := make(map[string]int, len(previous))
	currentCounts := make(map[string]int, len(current))
	for _, object := range previous {
		previousCounts[name(object)]++
	}
	for _, object := range current {
		currentCounts[name(object)]++
	}

	claimed := make(map[string]string)
	for _, object := range current {
		currentName := name(object)
		oldName := previousName(object)
		if oldName == "" {
			continue
		}
		if oldName == currentName {
			return fmt.Errorf("%s %q declares previousName equal to its current name %q", kind, currentName, oldName)
		}
		if previousCounts[oldName] == 0 {
			return fmt.Errorf("%s %q declares previousName %q but no such object exists in the previous snapshot", kind, currentName, oldName)
		}
		if previousCounts[oldName] > 1 {
			return fmt.Errorf("%s %q declares ambiguous previousName %q because multiple previous objects share that name", kind, currentName, oldName)
		}
		if previousCounts[currentName] > 0 {
			return fmt.Errorf("%s rename %q -> %q targets a name that already exists in the previous snapshot", kind, oldName, currentName)
		}
		if currentCounts[oldName] > 0 {
			return fmt.Errorf("%s rename from %q is ambiguous because the previous object remains in the current snapshot", kind, oldName)
		}
		if other, exists := claimed[oldName]; exists {
			return fmt.Errorf("%s previousName %q is claimed by both %q and %q", kind, oldName, other, currentName)
		}
		claimed[oldName] = currentName
	}
	return nil
}
