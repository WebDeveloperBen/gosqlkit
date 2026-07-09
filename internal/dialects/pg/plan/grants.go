package plan

import (
	"reflect"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func (p planner) grants(previous, current []pgschema.Grant) error {
	prev := mapBy(previous, grantIdentityKey)
	for _, grant := range sortedBy(current, grantIdentityKey) {
		key := grantIdentityKey(grant)
		old, ok := prev[key]
		if !ok {
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindGrant, key),
					"grant privileges on "+grantTargetSummary(grant.Target),
					migrateplan.SQL(renderGrant(grant)),
				).WithDependencies(
					grantDependencyRefs(grant)...,
				).WithReverse(
					migrateplan.SQL(renderRevokeGrant(grant)),
				),
			)
			continue
		}
		if !reflect.DeepEqual(normaliseGrant(old), normaliseGrant(grant)) {
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationReplace,
					migrateplan.Ref(migrateplan.ObjectKindGrant, key),
					"replace grant on "+grantTargetSummary(grant.Target),
				).WithRisks(
					migrateplan.RiskManualReview,
					migrateplan.RiskDestructive,
					migrateplan.RiskRequiresDDLReview,
				),
			)
		}
		delete(prev, key)
	}
	for _, key := range sortedStrings(removedNames(prev)) {
		p.addWith(
			migrateplan.NewChange(
				migrateplan.OperationDrop,
				migrateplan.Ref(migrateplan.ObjectKindGrant, key),
				"drop grant "+key,
			).WithRisks(
				migrateplan.RiskManualReview,
				migrateplan.RiskDestructive,
				migrateplan.RiskRequiresDDLReview,
			),
		)
	}
	return nil
}

func grantIdentityKey(grant pgschema.Grant) string {
	parts := []string{grant.Target.Type, grant.Target.Schema, grant.Target.Name}
	if grant.Target.AllInSchema {
		parts = append(parts, "all")
	}
	parts = append(parts, strings.Join(sortedStrings(grant.Grantees), ","))
	return strings.Join(parts, ":")
}

func normaliseGrant(grant pgschema.Grant) pgschema.Grant {
	grant.Privileges = append([]pgschema.GrantPrivilege(nil), grant.Privileges...)
	for i := range grant.Privileges {
		grant.Privileges[i].Name = strings.ToUpper(grant.Privileges[i].Name)
		grant.Privileges[i].Columns = sortedStrings(grant.Privileges[i].Columns)
	}
	grant.Grantees = sortedStrings(grant.Grantees)
	return grant
}

func grantDependencyRefs(grant pgschema.Grant) []migrateplan.ObjectRef {
	refs := make([]migrateplan.ObjectRef, 0, len(grant.Grantees)+1)
	for _, grantee := range grant.Grantees {
		if strings.EqualFold(grantee, "public") {
			continue
		}
		refs = append(refs, migrateplan.Ref(migrateplan.ObjectKindRole, grantee))
	}
	switch grant.Target.Type {
	case "schema":
		refs = append(refs, migrateplan.Ref(migrateplan.ObjectKindSchema, grant.Target.Name))
	case "sequence":
		if !grant.Target.AllInSchema {
			refs = append(refs, migrateplan.Ref(migrateplan.ObjectKindSequence, referencedTableKey(grant.Target.Name)))
		}
	case "function":
		if !grant.Target.AllInSchema {
			refs = append(refs, migrateplan.Ref(migrateplan.ObjectKindFunction, grantFunctionKey(grant.Target.Name)))
		}
	case "type":
		key := referencedTableKey(grant.Target.Name)
		refs = append(
			refs,
			migrateplan.Ref(migrateplan.ObjectKindEnum, key),
			migrateplan.Ref(migrateplan.ObjectKindCompositeType, key),
			migrateplan.Ref(migrateplan.ObjectKindDomain, key),
		)
	case "table":
		if !grant.Target.AllInSchema {
			refs = append(refs, migrateplan.Ref(migrateplan.ObjectKindTable, referencedTableKey(grant.Target.Name)))
		}
	}
	return uniqueRefs(refs)
}

func grantFunctionKey(target string) string {
	name, signature, ok := strings.Cut(target, "(")
	if !ok {
		return referencedTableKey(target)
	}
	return referencedTableKey(name) + "(" + signature
}

func grantTargetSummary(target pgschema.GrantTarget) string {
	if target.AllInSchema {
		return "all " + target.Type + "s in schema " + target.Schema
	}
	return target.Type + " " + target.Name
}
