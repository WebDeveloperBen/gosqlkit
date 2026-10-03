package sqliteschema

type TableMetadata struct {
	Source  string `json:"source,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type ColumnMetadata struct {
	Source string `json:"source,omitempty"`
}

type ViewMetadata struct {
	Source  string `json:"source,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type TriggerMetadata struct {
	Source string `json:"source,omitempty"`
}

func buildTableMetadata(tables []Table) map[string]TableMetadata {
	if len(tables) == 0 {
		return nil
	}
	m := make(map[string]TableMetadata, len(tables))
	for _, t := range tables {
		m[t.Name] = TableMetadata{Comment: t.Comment}
	}
	return m
}

func buildColumnMetadata(tables []Table) map[string]ColumnMetadata {
	total := 0
	for _, t := range tables {
		total += len(t.Columns)
	}
	if total == 0 {
		return nil
	}
	m := make(map[string]ColumnMetadata, total)
	for _, t := range tables {
		for _, col := range t.Columns {
			m[t.Name+"."+col.Name] = ColumnMetadata{}
		}
	}
	return m
}

func buildViewMetadata(views []View) map[string]ViewMetadata {
	if len(views) == 0 {
		return nil
	}
	m := make(map[string]ViewMetadata, len(views))
	for _, v := range views {
		m[v.Name] = ViewMetadata{Comment: v.Comment}
	}
	return m
}

func buildTriggerMetadata(triggers []Trigger) map[string]TriggerMetadata {
	if len(triggers) == 0 {
		return nil
	}
	m := make(map[string]TriggerMetadata, len(triggers))
	for _, trigger := range triggers {
		m[triggerKey(trigger)] = TriggerMetadata{}
	}
	return m
}
