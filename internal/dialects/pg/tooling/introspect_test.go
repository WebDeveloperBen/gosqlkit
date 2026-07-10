package tooling

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestParseIndexColumn(t *testing.T) {
	tests := []struct {
		want  string
		input string
	}{
		{input: "email", want: "email||/"},
		{input: "description text_ops", want: "description|text_ops|/"},
		{input: "created_at DESC NULLS LAST", want: "created_at||DESC/LAST"},
		{input: "description text_ops ASC NULLS FIRST", want: "description|text_ops|ASC/FIRST"},
	}

	for _, tt := range tests {
		got := parseIndexColumn(tt.input, false, 0)
		gotKey := got.Expression + "|" + got.OpClass + "|" + got.Order + "/" + got.Nulls
		if gotKey != tt.want {
			t.Fatalf("parseIndexColumn(%q) = %q, want %q", tt.input, gotKey, tt.want)
		}
	}
}

func TestParseIndexExpressionColumn(t *testing.T) {
	got := parseIndexColumn("lower(email) DESC", true, 0)
	if got.Expression != "lower(email)" || got.Order != "DESC" || !got.IsExpression {
		t.Fatalf("parseIndexColumn expression = %#v", got)
	}
}

func TestParseIndexColumnOptionBits(t *testing.T) {
	got := parseIndexColumn("created_at", false, 1)
	if got.Order != "DESC" || got.Nulls != "LAST" {
		t.Fatalf("parseIndexColumn options = %#v", got)
	}
}

func TestReloptionsMap(t *testing.T) {
	got := reloptionsMap([]string{"fillfactor=90", "deduplicate_items=off"})
	if got["fillfactor"] != "90" || got["deduplicate_items"] != "off" {
		t.Fatalf("reloptionsMap = %#v", got)
	}
}

func TestTypeObjectIntrospectionFiltersExtensionOwnedTypes(t *testing.T) {
	queryer := &recordingQueryer{}
	if _, err := introspectEnums(context.Background(), queryer); err != nil {
		t.Fatal(err)
	}
	if _, err := introspectCompositeTypes(context.Background(), queryer); err != nil {
		t.Fatal(err)
	}
	if _, err := introspectDomains(context.Background(), queryer); err != nil {
		t.Fatal(err)
	}

	if len(queryer.queries) != 3 {
		t.Fatalf("queries len = %d, want 3", len(queryer.queries))
	}
	for _, query := range queryer.queries {
		for _, want := range []string{
			"FROM pg_depend dep",
			"dep.classid = 'pg_type'::regclass",
			"dep.objid = t.oid",
			"dep.deptype = 'e'",
		} {
			if !strings.Contains(query, want) {
				t.Fatalf("query missing %q:\n%s", want, query)
			}
		}
	}
}

func TestParsePartitioning(t *testing.T) {
	got, ok, err := parsePartitioning("RANGE (created_at, tenant_id)")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Strategy != "range" || len(got.Keys) != 2 {
		t.Fatalf("partitioning = %#v", got)
	}
	if got.Keys[0].Expression != "created_at" || got.Keys[0].IsExpression {
		t.Fatalf("first key = %#v", got.Keys[0])
	}
	if got.Keys[1].Expression != "tenant_id" || got.Keys[1].IsExpression {
		t.Fatalf("second key = %#v", got.Keys[1])
	}
}

func TestParsePartitioningExpressionKey(t *testing.T) {
	got, ok, err := parsePartitioning("HASH ((lower(email)))")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Strategy != "hash" || len(got.Keys) != 1 {
		t.Fatalf("partitioning = %#v", got)
	}
	if got.Keys[0].Expression != "lower(email)" || !got.Keys[0].IsExpression {
		t.Fatalf("expression key = %#v", got.Keys[0])
	}
}

func TestParsePartitionRangeBound(t *testing.T) {
	got, ok, err := parsePartitionBound("FOR VALUES FROM ('0') TO ('10')")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Type != "range" || len(got.From) != 1 || got.From[0] != "0" || len(got.To) != 1 || got.To[0] != "10" {
		t.Fatalf("range bound = %#v", got)
	}
}

func TestParsePartitionListBound(t *testing.T) {
	got, ok, err := parsePartitionBound("FOR VALUES IN ('draft', 'issued')")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Type != "list" || len(got.Values) != 2 || got.Values[0] != "'draft'" || got.Values[1] != "'issued'" {
		t.Fatalf("list bound = %#v", got)
	}
}

func TestParsePartitionHashBound(t *testing.T) {
	got, ok, err := parsePartitionBound("FOR VALUES WITH (modulus 4, remainder 2)")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Type != "hash" || got.Modulus != 4 || got.Remainder != 2 {
		t.Fatalf("hash bound = %#v", got)
	}
}

func TestParsePartitionDefaultBound(t *testing.T) {
	got, ok, err := parsePartitionBound("DEFAULT")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Type != "default" {
		t.Fatalf("default bound = %#v", got)
	}
}

type recordingQueryer struct {
	queries []string
}

func (q *recordingQueryer) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	q.queries = append(q.queries, sql)
	return emptyRows{}, nil
}

type emptyRows struct{}

func (emptyRows) Close() {}

func (emptyRows) Err() error {
	return nil
}

func (emptyRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (emptyRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (emptyRows) Next() bool {
	return false
}

func (emptyRows) Scan(...any) error {
	return nil
}

func (emptyRows) Values() ([]any, error) {
	return nil, nil
}

func (emptyRows) RawValues() [][]byte {
	return nil
}

func (emptyRows) Conn() *pgx.Conn {
	return nil
}
