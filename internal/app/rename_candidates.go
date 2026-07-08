package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func planWithRenameDecisions(planner migrateplan.SnapshotPlanner, previous []byte, snapshot string, planned *migrateplan.Plan, decide RenameDecisionFunc) (*migrateplan.Plan, error) {
	candidates := renameCandidatesFromPlan(planned)
	if len(candidates) == 0 {
		return planned, nil
	}
	decisions, err := decide(candidates)
	if err != nil {
		return nil, err
	}
	accepted, err := acceptedRenameCandidates(decisions)
	if err != nil {
		return nil, err
	}
	if len(accepted) == 0 {
		return planned, nil
	}
	annotated, err := applyRenameCandidatesToSnapshot(snapshot, accepted)
	if err != nil {
		return nil, err
	}
	replanned, err := planner.PlanSnapshotDiff(previous, annotated)
	if err != nil {
		return nil, err
	}
	return replanned, nil
}

func renameCandidatesFromPlan(plan *migrateplan.Plan) []RenameCandidate {
	if plan == nil {
		return nil
	}
	removed := map[string][]migrateplan.Change{}
	added := map[string][]migrateplan.Change{}
	for _, change := range plan.Changes {
		if !renameableKind(change.Object.Kind) {
			continue
		}
		group := renameCandidateGroup(change.Object.Kind, change.Object.Key)
		switch {
		case isRenameCandidateRemoval(change):
			removed[group] = append(removed[group], change)
		case isRenameCandidateAddition(change):
			added[group] = append(added[group], change)
		}
	}

	candidates := make([]RenameCandidate, 0)
	for group, newChanges := range added {
		oldChanges := removed[group]
		for _, oldChange := range oldChanges {
			for _, newChange := range newChanges {
				if oldChange.Object.Key == newChange.Object.Key {
					continue
				}
				candidates = append(candidates, newRenameCandidate(oldChange.Object.Kind, oldChange.Object.Key, newChange.Object.Key))
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Kind == candidates[j].Kind {
			if candidates[i].ParentKey == candidates[j].ParentKey {
				if candidates[i].FromKey == candidates[j].FromKey {
					return candidates[i].ToKey < candidates[j].ToKey
				}
				return candidates[i].FromKey < candidates[j].FromKey
			}
			return candidates[i].ParentKey < candidates[j].ParentKey
		}
		return candidates[i].Kind < candidates[j].Kind
	})
	return candidates
}

func acceptedRenameCandidates(decisions []RenameDecision) ([]RenameCandidate, error) {
	accepted := make([]RenameCandidate, 0, len(decisions))
	from := map[string]struct{}{}
	to := map[string]struct{}{}
	for _, decision := range decisions {
		if !decision.Accept {
			continue
		}
		if _, ok := from[decision.Candidate.FromKey]; ok {
			return nil, fmt.Errorf("rename candidate %s is selected more than once as a source", decision.Candidate.FromKey)
		}
		if _, ok := to[decision.Candidate.ToKey]; ok {
			return nil, fmt.Errorf("rename candidate %s is selected more than once as a target", decision.Candidate.ToKey)
		}
		from[decision.Candidate.FromKey] = struct{}{}
		to[decision.Candidate.ToKey] = struct{}{}
		accepted = append(accepted, decision.Candidate)
	}
	return accepted, nil
}

func isRenameCandidateRemoval(change migrateplan.Change) bool {
	if change.Op == migrateplan.OperationDrop {
		return true
	}
	if !strings.HasPrefix(change.Summary, "drop ") {
		return false
	}
	return change.HasRisk(migrateplan.RiskDestructive) || change.HasRisk(migrateplan.RiskDataLoss)
}

func isRenameCandidateAddition(change migrateplan.Change) bool {
	if change.HasRisk(migrateplan.RiskDestructive) || change.HasRisk(migrateplan.RiskDataLoss) {
		return false
	}
	return strings.HasPrefix(change.Summary, "create ") || strings.HasPrefix(change.Summary, "add ")
}

func renameableKind(kind migrateplan.ObjectKind) bool {
	switch kind {
	case migrateplan.ObjectKindSchema,
		migrateplan.ObjectKindEnum,
		migrateplan.ObjectKindCompositeType,
		migrateplan.ObjectKindDomain,
		migrateplan.ObjectKindSequence,
		migrateplan.ObjectKindRole,
		migrateplan.ObjectKindFunction,
		migrateplan.ObjectKindTable,
		migrateplan.ObjectKindColumn,
		migrateplan.ObjectKindConstraint,
		migrateplan.ObjectKindIndex,
		migrateplan.ObjectKindView,
		migrateplan.ObjectKindMaterializedView,
		migrateplan.ObjectKindTrigger,
		migrateplan.ObjectKindPolicy:
		return true
	default:
		return false
	}
}

func renameCandidateGroup(kind migrateplan.ObjectKind, key string) string {
	switch kind {
	case migrateplan.ObjectKindColumn,
		migrateplan.ObjectKindConstraint,
		migrateplan.ObjectKindIndex,
		migrateplan.ObjectKindTrigger,
		migrateplan.ObjectKindPolicy:
		return string(kind) + ":" + parentObjectKey(key)
	default:
		return string(kind)
	}
}

func newRenameCandidate(kind migrateplan.ObjectKind, fromKey, toKey string) RenameCandidate {
	return RenameCandidate{
		Kind:      string(kind),
		FromKey:   fromKey,
		ToKey:     toKey,
		FromName:  objectLocalName(fromKey),
		ToName:    objectLocalName(toKey),
		ParentKey: parentKeyForRenameCandidate(kind, toKey),
	}
}

func parentKeyForRenameCandidate(kind migrateplan.ObjectKind, key string) string {
	switch kind {
	case migrateplan.ObjectKindColumn,
		migrateplan.ObjectKindConstraint,
		migrateplan.ObjectKindIndex,
		migrateplan.ObjectKindTrigger,
		migrateplan.ObjectKindPolicy:
		return parentObjectKey(key)
	default:
		return ""
	}
}

func parentObjectKey(key string) string {
	idx := strings.LastIndex(key, ".")
	if idx <= 0 {
		return ""
	}
	return key[:idx]
}

func applyRenameCandidatesToSnapshot(snapshot string, candidates []RenameCandidate) ([]byte, error) {
	var doc pgschema.Document
	if err := json.Unmarshal([]byte(snapshot), &doc); err != nil {
		return nil, fmt.Errorf("parse current snapshot for rename decisions: %w", err)
	}
	for _, candidate := range candidates {
		if err := applyRenameCandidate(&doc, candidate); err != nil {
			return nil, err
		}
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func applyRenameCandidate(doc *pgschema.Document, candidate RenameCandidate) error {
	previousName := objectLocalName(candidate.FromKey)
	switch migrateplan.ObjectKind(candidate.Kind) {
	case migrateplan.ObjectKindSchema:
		for i := range doc.Namespaces {
			if doc.Namespaces[i].Name == candidate.ToKey {
				doc.Namespaces[i].PreviousName = candidate.FromKey
				return nil
			}
		}
	case migrateplan.ObjectKindRole:
		for i := range doc.Roles {
			if doc.Roles[i].Name == candidate.ToKey {
				doc.Roles[i].PreviousName = candidate.FromKey
				return nil
			}
		}
	case migrateplan.ObjectKindTable:
		for i := range doc.Tables {
			if qualifiedSnapshotName(doc.Tables[i].Schema, doc.Tables[i].Name) == candidate.ToKey {
				doc.Tables[i].PreviousName = previousName
				return nil
			}
		}
	case migrateplan.ObjectKindColumn:
		for i := range doc.Tables {
			if qualifiedSnapshotName(doc.Tables[i].Schema, doc.Tables[i].Name) != candidate.ParentKey {
				continue
			}
			for j := range doc.Tables[i].Columns {
				if doc.Tables[i].Columns[j].Name == candidate.ToName {
					doc.Tables[i].Columns[j].PreviousName = previousName
					return nil
				}
			}
		}
	case migrateplan.ObjectKindConstraint:
		return applyConstraintRenameCandidate(doc, candidate, previousName)
	case migrateplan.ObjectKindIndex:
		for i := range doc.Tables {
			if qualifiedSnapshotName(doc.Tables[i].Schema, doc.Tables[i].Name) != candidate.ParentKey {
				continue
			}
			for j := range doc.Tables[i].Indexes {
				if doc.Tables[i].Indexes[j].Name == candidate.ToName {
					doc.Tables[i].Indexes[j].PreviousName = previousName
					return nil
				}
			}
		}
	case migrateplan.ObjectKindEnum:
		for i := range doc.Enums {
			if qualifiedSnapshotName(doc.Enums[i].Schema, doc.Enums[i].Name) == candidate.ToKey {
				doc.Enums[i].PreviousName = previousName
				return nil
			}
		}
	case migrateplan.ObjectKindCompositeType:
		for i := range doc.CompositeTypes {
			if qualifiedSnapshotName(doc.CompositeTypes[i].Schema, doc.CompositeTypes[i].Name) == candidate.ToKey {
				doc.CompositeTypes[i].PreviousName = previousName
				return nil
			}
		}
	case migrateplan.ObjectKindDomain:
		for i := range doc.Domains {
			if qualifiedSnapshotName(doc.Domains[i].Schema, doc.Domains[i].Name) == candidate.ToKey {
				doc.Domains[i].PreviousName = previousName
				return nil
			}
		}
	case migrateplan.ObjectKindSequence:
		for i := range doc.Sequences {
			if qualifiedSnapshotName(doc.Sequences[i].Schema, doc.Sequences[i].Name) == candidate.ToKey {
				doc.Sequences[i].PreviousName = previousName
				return nil
			}
		}
	case migrateplan.ObjectKindFunction:
		for i := range doc.Functions {
			if snapshotFunctionKey(doc.Functions[i]) == candidate.ToKey {
				doc.Functions[i].PreviousName = objectLocalNameWithoutSignature(candidate.FromKey)
				return nil
			}
		}
	case migrateplan.ObjectKindView:
		for i := range doc.Views {
			if qualifiedSnapshotName(doc.Views[i].Schema, doc.Views[i].Name) == candidate.ToKey {
				doc.Views[i].PreviousName = previousName
				return nil
			}
		}
	case migrateplan.ObjectKindMaterializedView:
		for i := range doc.MaterializedViews {
			if qualifiedSnapshotName(doc.MaterializedViews[i].Schema, doc.MaterializedViews[i].Name) == candidate.ToKey {
				doc.MaterializedViews[i].PreviousName = previousName
				return nil
			}
		}
	case migrateplan.ObjectKindTrigger:
		for i := range doc.Triggers {
			if snapshotTriggerKey(doc.Triggers[i]) == candidate.ToKey {
				doc.Triggers[i].PreviousName = previousName
				return nil
			}
		}
	case migrateplan.ObjectKindPolicy:
		for i := range doc.Policies {
			if snapshotPolicyKey(doc.Policies[i]) == candidate.ToKey {
				doc.Policies[i].PreviousName = previousName
				return nil
			}
		}
	}
	return fmt.Errorf("rename candidate target %s %s was not found in current snapshot", candidate.Kind, candidate.ToKey)
}

func applyConstraintRenameCandidate(doc *pgschema.Document, candidate RenameCandidate, previousName string) error {
	for i := range doc.Tables {
		if qualifiedSnapshotName(doc.Tables[i].Schema, doc.Tables[i].Name) != candidate.ParentKey {
			continue
		}
		for j := range doc.Tables[i].PrimaryKeys {
			if doc.Tables[i].PrimaryKeys[j].Name == candidate.ToName {
				doc.Tables[i].PrimaryKeys[j].PreviousName = previousName
				return nil
			}
		}
		for j := range doc.Tables[i].UniqueConstraints {
			if doc.Tables[i].UniqueConstraints[j].Name == candidate.ToName {
				doc.Tables[i].UniqueConstraints[j].PreviousName = previousName
				return nil
			}
		}
		for j := range doc.Tables[i].ForeignKeys {
			if doc.Tables[i].ForeignKeys[j].Name == candidate.ToName {
				doc.Tables[i].ForeignKeys[j].PreviousName = previousName
				return nil
			}
		}
		for j := range doc.Tables[i].Checks {
			if doc.Tables[i].Checks[j].Name == candidate.ToName {
				doc.Tables[i].Checks[j].PreviousName = previousName
				return nil
			}
		}
		for j := range doc.Tables[i].Exclusions {
			if doc.Tables[i].Exclusions[j].Name == candidate.ToName {
				doc.Tables[i].Exclusions[j].PreviousName = previousName
				return nil
			}
		}
	}
	return fmt.Errorf("rename candidate target %s %s was not found in current snapshot", candidate.Kind, candidate.ToKey)
}

func qualifiedSnapshotName(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}

func snapshotFunctionKey(function pgschema.Function) string {
	parts := make([]string, 0, len(function.Arguments))
	for _, arg := range function.Arguments {
		if strings.EqualFold(arg.Mode, "OUT") {
			continue
		}
		parts = append(parts, arg.Type)
	}
	return qualifiedSnapshotName(function.Schema, function.Name) + "(" + strings.Join(parts, ", ") + ")"
}

func snapshotTriggerKey(trigger pgschema.Trigger) string {
	return referencedSnapshotObjectKey(trigger.Target) + "." + trigger.Name
}

func snapshotPolicyKey(policy pgschema.Policy) string {
	return referencedSnapshotObjectKey(policy.Table) + "." + policy.Name
}

func referencedSnapshotObjectKey(value string) string {
	if strings.Contains(value, ".") {
		return value
	}
	return qualifiedSnapshotName("", value)
}

func objectLocalNameWithoutSignature(key string) string {
	if idx := strings.Index(key, "("); idx >= 0 {
		key = key[:idx]
	}
	return objectLocalName(key)
}
