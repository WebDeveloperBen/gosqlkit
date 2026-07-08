package tooling

import "testing"

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
