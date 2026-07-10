package plan

import "strings"

// RenderRefreshMaterializedView renders a REFRESH MATERIALIZED VIEW statement.
// Refresh is a data-population operation rather than structural schema, so it
// is authored as an explicit migration step instead of being embedded in the
// materialized view's snapshot or CREATE rendering.
func RenderRefreshMaterializedView(schema, name string, concurrently bool) string {
	var b strings.Builder
	b.WriteString("REFRESH MATERIALIZED VIEW ")
	if concurrently {
		b.WriteString("CONCURRENTLY ")
	}
	b.WriteString(renderQualified(schema, name))
	b.WriteString(";")
	return b.String()
}
