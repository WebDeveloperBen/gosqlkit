package pgschema

import (
	"encoding/json"
)

const SnapshotVersion = 1

type Schema struct {
	Namespaces        []Namespace        `json:"namespaces,omitempty"`
	Extensions        []Extension        `json:"extensions,omitempty"`
	Roles             []Role             `json:"roles,omitempty"`
	Collations        []Collation        `json:"collations,omitempty"`
	Enums             []Enum             `json:"enums,omitempty"`
	CompositeTypes    []CompositeType    `json:"compositeTypes,omitempty"`
	Domains           []Domain           `json:"domains,omitempty"`
	Sequences         []Sequence         `json:"sequences,omitempty"`
	Functions         []Function         `json:"functions,omitempty"`
	Tables            []Table            `json:"tables,omitempty"`
	Views             []View             `json:"views,omitempty"`
	MaterializedViews []MaterializedView `json:"materializedViews,omitempty"`
	Triggers          []Trigger          `json:"triggers,omitempty"`
	Policies          []Policy           `json:"policies,omitempty"`
	Grants            []Grant            `json:"grants,omitempty"`
}

type Document struct {
	ColumnMetadata     map[string]ColumnMetadata   `json:"columnMetadata,omitempty"`
	TableMetadata      map[string]TableMetadata    `json:"tableMetadata,omitempty"`
	ViewMetadata       map[string]ViewMetadata     `json:"viewMetadata,omitempty"`
	RoleMetadata       map[string]RoleMetadata     `json:"roleMetadata,omitempty"`
	FunctionMetadata   map[string]FunctionMetadata `json:"functionMetadata,omitempty"`
	TriggerMetadata    map[string]TriggerMetadata  `json:"triggerMetadata,omitempty"`
	PolicyMetadata     map[string]PolicyMetadata   `json:"policyMetadata,omitempty"`
	SchemaMetadata     map[string]SchemaMetadata   `json:"schemaMetadata,omitempty"`
	SnapshotID         string                      `json:"snapshotId,omitempty"`
	PreviousSnapshotID string                      `json:"previousSnapshotId,omitempty"`
	Dialect            string                      `json:"dialect"`
	Namespaces         []Namespace                 `json:"namespaces,omitempty"`
	Extensions         []Extension                 `json:"extensions,omitempty"`
	Roles              []Role                      `json:"roles,omitempty"`
	Collations         []Collation                 `json:"collations,omitempty"`
	Enums              []Enum                      `json:"enums,omitempty"`
	CompositeTypes     []CompositeType             `json:"compositeTypes,omitempty"`
	Domains            []Domain                    `json:"domains,omitempty"`
	Sequences          []Sequence                  `json:"sequences,omitempty"`
	Functions          []Function                  `json:"functions,omitempty"`
	Tables             []Table                     `json:"tables,omitempty"`
	Views              []View                      `json:"views,omitempty"`
	MaterializedViews  []MaterializedView          `json:"materializedViews,omitempty"`
	Triggers           []Trigger                   `json:"triggers,omitempty"`
	Policies           []Policy                    `json:"policies,omitempty"`
	Grants             []Grant                     `json:"grants,omitempty"`
	Version            int                         `json:"version"`
}

func JSON(dialect string, schema Schema) ([]byte, error) {
	doc := Document{
		Dialect:           dialect,
		Version:           SnapshotVersion,
		Namespaces:        sortedNamespaces(schema.Namespaces),
		Extensions:        sortedExtensions(schema.Extensions),
		Roles:             sortedRoles(schema.Roles),
		Collations:        sortedCollations(schema.Collations),
		Enums:             sortedEnums(schema.Enums),
		CompositeTypes:    sortedCompositeTypes(schema.CompositeTypes),
		Domains:           sortedDomains(schema.Domains),
		Sequences:         sortedSequences(schema.Sequences),
		Functions:         sortedFunctions(schema.Functions),
		Tables:            sortedTables(schema.Tables),
		Views:             sortedViews(schema.Views),
		MaterializedViews: sortedMaterializedViews(schema.MaterializedViews),
		Triggers:          sortedTriggers(schema.Triggers),
		Policies:          sortedPolicies(schema.Policies),
		Grants:            sortedGrants(schema.Grants),
		SchemaMetadata:    buildSchemaMetadata(schema.Namespaces),
		TableMetadata:     buildTableMetadata(schema.Tables),
		ColumnMetadata:    buildColumnMetadata(schema.Tables),
		ViewMetadata:      buildViewMetadata(schema.Views, schema.MaterializedViews),
		RoleMetadata:      buildRoleMetadata(schema.Roles),
		FunctionMetadata:  buildFunctionMetadata(schema.Functions),
		TriggerMetadata:   buildTriggerMetadata(schema.Triggers),
		PolicyMetadata:    buildPolicyMetadata(schema.Policies),
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func qualified(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}
