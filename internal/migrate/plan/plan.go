package plan

import (
	"encoding/json"
	"errors"
)

type Operation string

const (
	OperationCreate  Operation = "create"
	OperationAlter   Operation = "alter"
	OperationDrop    Operation = "drop"
	OperationRename  Operation = "rename"
	OperationReplace Operation = "replace"
)

type Risk string

const (
	RiskManualReview      Risk = "manual-review"
	RiskDestructive       Risk = "destructive"
	RiskDataLoss          Risk = "data-loss"
	RiskLockHeavy         Risk = "lock-heavy"
	RiskNonTransactional  Risk = "non-transactional"
	RiskRequiresBackfill  Risk = "requires-backfill"
	RiskRequiresDDLReview Risk = "requires-ddl-review"
)

type ObjectKind string

const (
	ObjectKindSchema           ObjectKind = "schema"
	ObjectKindExtension        ObjectKind = "extension"
	ObjectKindCollation        ObjectKind = "collation"
	ObjectKindEnum             ObjectKind = "enum"
	ObjectKindCompositeType    ObjectKind = "composite_type"
	ObjectKindDomain           ObjectKind = "domain"
	ObjectKindSequence         ObjectKind = "sequence"
	ObjectKindRole             ObjectKind = "role"
	ObjectKindFunction         ObjectKind = "function"
	ObjectKindTable            ObjectKind = "table"
	ObjectKindColumn           ObjectKind = "column"
	ObjectKindConstraint       ObjectKind = "constraint"
	ObjectKindIndex            ObjectKind = "index"
	ObjectKindView             ObjectKind = "view"
	ObjectKindMaterializedView ObjectKind = "materialized_view"
	ObjectKindTrigger          ObjectKind = "trigger"
	ObjectKindPolicy           ObjectKind = "policy"
	ObjectKindGrant            ObjectKind = "grant"
	ObjectKindRawSQL           ObjectKind = "raw_sql"
)

type ObjectRef struct {
	Kind ObjectKind `json:"kind"`
	Key  string     `json:"key"`
}

type Statement struct {
	SQL string `json:"sql"`
}

type Change struct {
	Summary           string      `json:"summary,omitempty"`
	Object            ObjectRef   `json:"object"`
	Op                Operation   `json:"op"`
	Risks             []Risk      `json:"risks,omitempty"`
	Dependencies      []ObjectRef `json:"dependencies,omitempty"`
	Statements        []Statement `json:"statements,omitempty"`
	ReverseStatements []Statement `json:"reverseStatements,omitempty"`
	Reversible        bool        `json:"reversible,omitempty"`
}

type Plan struct {
	Changes    []Change `json:"changes"`
	Statements []string `json:"statements,omitempty"`
}

func (p *Plan) DestructiveChanges() []Change {
	if p == nil {
		return nil
	}
	out := make([]Change, 0)
	for _, change := range p.Changes {
		for _, risk := range change.Risks {
			if risk == RiskDestructive {
				out = append(out, change)
				break
			}
		}
	}
	return out
}

func (p *Plan) HasDestructive() bool {
	return len(p.DestructiveChanges()) > 0
}

func (p *Plan) DataLossChanges() []Change {
	if p == nil {
		return nil
	}
	out := make([]Change, 0)
	for _, change := range p.Changes {
		for _, risk := range change.Risks {
			if risk == RiskDataLoss {
				out = append(out, change)
				break
			}
		}
	}
	return out
}

func (p *Plan) HasDataLoss() bool {
	return len(p.DataLossChanges()) > 0
}

type Summary struct {
	Changes []Change `json:"changes"`
}

type SnapshotPlanner interface {
	Dialect() string
	PlanSnapshotDiff(previousJSON, currentJSON []byte) (*Plan, error)
}

func Ref(kind ObjectKind, key string) ObjectRef {
	return ObjectRef{Kind: kind, Key: key}
}

func SQL(sql string) Statement {
	return Statement{SQL: sql}
}

func NewChange(op Operation, object ObjectRef, summary string, statements ...Statement) Change {
	return Change{
		Op:         op,
		Object:     object,
		Summary:    summary,
		Statements: append([]Statement(nil), statements...),
	}
}

func (c Change) WithDependencies(dependencies ...ObjectRef) Change {
	c.Dependencies = append(c.Dependencies, dependencies...)
	return c
}

func (c Change) WithRisks(risks ...Risk) Change {
	c.Risks = append(c.Risks, risks...)
	return c
}

func (c Change) WithReverse(statements ...Statement) Change {
	c.ReverseStatements = append(c.ReverseStatements, statements...)
	c.Reversible = len(c.ReverseStatements) > 0
	return c
}

func (c Change) HasRisk(risk Risk) bool {
	for _, r := range c.Risks {
		if r == risk {
			return true
		}
	}
	return false
}

func (c Change) RisksContainDestructive() bool {
	return c.HasRisk(RiskDestructive)
}

func (c Change) RisksContainDataLoss() bool {
	return c.HasRisk(RiskDataLoss)
}

func SummaryJSON(plan *Plan) ([]byte, error) {
	if plan == nil {
		return nil, errors.New("plan is required")
	}
	changes := plan.Changes
	if changes == nil {
		changes = []Change{}
	}
	data, err := json.MarshalIndent(Summary{Changes: changes}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
