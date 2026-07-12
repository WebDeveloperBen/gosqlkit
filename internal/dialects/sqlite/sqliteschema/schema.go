package sqliteschema

import "encoding/json"

const SnapshotVersion = 1

type Schema struct {
	Tables   []Table   `json:"tables,omitempty"`
	Views    []View    `json:"views,omitempty"`
	Triggers []Trigger `json:"triggers,omitempty"`
	RawSQL   []RawSQL  `json:"rawSQL,omitempty"`
}

type Document struct {
	ColumnMetadata     map[string]ColumnMetadata  `json:"columnMetadata,omitempty"`
	TableMetadata      map[string]TableMetadata   `json:"tableMetadata,omitempty"`
	ViewMetadata       map[string]ViewMetadata    `json:"viewMetadata,omitempty"`
	TriggerMetadata    map[string]TriggerMetadata `json:"triggerMetadata,omitempty"`
	SnapshotID         string                     `json:"snapshotId,omitempty"`
	PreviousSnapshotID string                     `json:"previousSnapshotId,omitempty"`
	Dialect            string                     `json:"dialect"`
	Tables             []Table                    `json:"tables,omitempty"`
	Views              []View                     `json:"views,omitempty"`
	Triggers           []Trigger                  `json:"triggers,omitempty"`
	RawSQL             []RawSQL                   `json:"rawSQL,omitempty"`
	Version            int                        `json:"version"`
}

func JSON(dialect string, schema Schema) ([]byte, error) {
	doc := Document{
		Dialect:         dialect,
		Version:         SnapshotVersion,
		Tables:          sortedTables(schema.Tables),
		Views:           sortedViews(schema.Views),
		Triggers:        sortedTriggers(schema.Triggers),
		RawSQL:          sortedRawSQL(schema.RawSQL),
		TableMetadata:   buildTableMetadata(schema.Tables),
		ColumnMetadata:  buildColumnMetadata(schema.Tables),
		ViewMetadata:    buildViewMetadata(schema.Views),
		TriggerMetadata: buildTriggerMetadata(schema.Triggers),
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func NormaliseDocumentForDiff(document Document) (Document, error) {
	raw, err := JSON(document.Dialect, Schema{
		Tables:   document.Tables,
		Views:    document.Views,
		Triggers: document.Triggers,
		RawSQL:   document.RawSQL,
	})
	if err != nil {
		return Document{}, err
	}

	var normalised Document
	if err := json.Unmarshal(raw, &normalised); err != nil {
		return Document{}, err
	}
	normalised.SnapshotID = ""
	normalised.PreviousSnapshotID = ""
	normalised.TableMetadata = nil
	normalised.ColumnMetadata = nil
	normalised.ViewMetadata = nil
	normalised.TriggerMetadata = nil
	return normalised, nil
}
