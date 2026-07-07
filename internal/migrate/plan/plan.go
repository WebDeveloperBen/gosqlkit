package plan

type Operation string

const (
	OperationCreate Operation = "create"
	OperationAlter  Operation = "alter"
)

type Risk string

const (
	RiskManualReview Risk = "manual-review"
	RiskDestructive  Risk = "destructive"
)

type Change struct {
	Object  string
	Summary string
	Op      Operation
	Risks   []Risk
}

type Plan struct {
	Changes    []Change
	Statements []string
}

type SnapshotPlanner interface {
	Dialect() string
	PlanSnapshotDiff(previousJSON, currentJSON []byte) (*Plan, error)
}
