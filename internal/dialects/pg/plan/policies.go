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
				oldPolicy, hasOld := prev[policyPreviousKey(policy)]
				if hasOld {
					if err := ensureRenameOnlyPolicy(policy, oldPolicy); err != nil {
						return err
					}
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindPolicy, key),
							"rename policy "+policyPreviousKey(policy)+" to "+key,
							migrateplan.SQL(renderRenamePolicy(policy.Table, policy.PreviousName, policy.Name)),
						).WithDependencies(
							migrateplan.Ref(migrateplan.ObjectKindTable, referencedTableKey(policy.Table)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenamePolicy(policy.Table, policy.Name, policy.PreviousName)),
						),
					)
					delete(prev, policyPreviousKey(policy))
					continue
				}
				return unsupported("policy " + key + " previousName " + policy.PreviousName + " does not match any policy in the previous snapshot")
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
				).WithReverse(
					migrateplan.SQL(renderPolicy(policy)),
				),
			)
		}
	}
	return nil
}
