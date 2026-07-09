package pgschema

type Partitioning struct {
	Strategy string         `json:"strategy"`
	Keys     []PartitionKey `json:"keys,omitempty"`
}

type PartitionKey struct {
	Expression   string `json:"expression"`
	IsExpression bool   `json:"isExpression,omitempty"`
}

type PartitionOf struct {
	Parent string         `json:"parent"`
	Bound  PartitionBound `json:"bound"`
}

type PartitionBound struct {
	Type      string   `json:"type"`
	From      []string `json:"from,omitempty"`
	To        []string `json:"to,omitempty"`
	Values    []string `json:"values,omitempty"`
	Modulus   int      `json:"modulus,omitempty"`
	Remainder int      `json:"remainder,omitempty"`
}
