package plan

import (
	"reflect"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func (p planner) policies(previous, current []pgschema.Policy) error {
	prev := mapBy(previous, policyKey)
	for _, policy := range sortedBy(current, policyKey) {
		key := policyKey(policy)
		old, ok := prev[key]
		if !ok {
			if policy.PreviousName != "" {
				return unsupported("policy " + key + " rename metadata requires semantic planning")
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindPolicy, key),
					"create policy "+key,
					migrateplan.SQL(renderPolicy(policy)),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, referencedTableKey(policy.Table)),
				).WithReverse(
					migrateplan.SQL("DROP POLICY " + policy.Name + " ON " + renderReferencedTable(policy.Table) + ";"),
				),
			)
			continue
		}
		if !reflect.DeepEqual(old, policy) {
			return unsupported("policy modifications require semantic planning")
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		for _, key := range sortedStrings(removedNames(prev)) {
			policy := prev[key]
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindPolicy, key),
					"drop policy "+key,
					migrateplan.SQL("DROP POLICY "+policy.Name+" ON "+renderReferencedTable(policy.Table)+";"),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, referencedTableKey(policy.Table)),
				).WithRisks(
					migrateplan.RiskDestructive,
				),
			)
		}
	}
	return nil
}
