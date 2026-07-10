package plan

import (
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func (p planner) rawSQL(previous, current []pgschema.RawSQL) error {
	prev := make(map[string]pgschema.RawSQL, len(previous))
	for _, block := range previous {
		prev[block.Name] = block
	}

	for _, block := range sortedRawSQLByName(current) {
		old, ok := prev[block.Name]
		if !ok {
			change := migrateplan.NewChange(
				migrateplan.OperationCreate,
				migrateplan.Ref(migrateplan.ObjectKindRawSQL, block.Name),
				"apply raw SQL block "+block.Name,
				migrateplan.SQL(strings.TrimSpace(block.SQL)),
			).WithRisks(
				migrateplan.RiskRequiresDDLReview,
			)
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
		names := make([]string, 0, len(prev))
		for name := range prev {
			names = append(names, name)
		}
		return unsupported("raw SQL block " + sortedStrings(names)[0] + " removal requires manual migration authoring; gosqlkit cannot infer how to reverse arbitrary SQL")
	}
	return nil
}

func sortedRawSQLByName(input []pgschema.RawSQL) []pgschema.RawSQL {
	items := append([]pgschema.RawSQL(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}
