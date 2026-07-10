package plan_test

import (
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
)

func TestRenderRefreshMaterializedView(t *testing.T) {
	cases := []struct {
		name         string
		schema       string
		view         string
		want         string
		concurrently bool
	}{
		{name: "plain", view: "cached_bookings", want: "REFRESH MATERIALIZED VIEW cached_bookings;"},
		{name: "concurrent", view: "cached_bookings", concurrently: true, want: "REFRESH MATERIALIZED VIEW CONCURRENTLY cached_bookings;"},
		{name: "qualified", schema: "reporting", view: "daily", want: "REFRESH MATERIALIZED VIEW reporting.daily;"},
		{name: "public normalised", schema: "public", view: "daily", want: "REFRESH MATERIALIZED VIEW daily;"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := plan.RenderRefreshMaterializedView(tc.schema, tc.view, tc.concurrently); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
