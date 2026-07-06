package pg_test

import (
	"testing"

	"github.com/webdeveloperben/gosqlkit/pg"
)

func TestRoleDSLRegistersOptions(t *testing.T) {
	pg.Reset()
	t.Cleanup(pg.Reset)

	pg.Role("app_user").
		Login().
		NoSuperuser().
		CreateDB().
		NoInherit().
		ConnectionLimit(10).
		ValidUntil("2030-01-01 00:00:00+00").
		MemberOf("app_readers", "app_writers").
		AdminOf("app_admins").
		PreviousName("old_app_user")

	schema := pg.Schema()
	if len(schema.Roles) != 1 {
		t.Fatalf("roles len = %d, want 1", len(schema.Roles))
	}

	role := schema.Roles[0]
	if role.Name != "app_user" || role.PreviousName != "old_app_user" {
		t.Fatalf("unexpected role identity %#v", role)
	}
	if role.Login == nil || !*role.Login {
		t.Fatalf("expected login role option, got %#v", role.Login)
	}
	if role.Superuser == nil || *role.Superuser {
		t.Fatalf("expected no-superuser role option, got %#v", role.Superuser)
	}
	if role.CreateDB == nil || !*role.CreateDB {
		t.Fatalf("expected createdb role option, got %#v", role.CreateDB)
	}
	if role.Inherit == nil || *role.Inherit {
		t.Fatalf("expected noinherit role option, got %#v", role.Inherit)
	}
	if role.ConnectionLimit == nil || *role.ConnectionLimit != 10 {
		t.Fatalf("unexpected connection limit %#v", role.ConnectionLimit)
	}
	if role.ValidUntil != "2030-01-01 00:00:00+00" {
		t.Fatalf("valid until = %q", role.ValidUntil)
	}
	if len(role.MemberOf) != 2 || role.MemberOf[0] != "app_readers" || role.MemberOf[1] != "app_writers" {
		t.Fatalf("unexpected memberOf %#v", role.MemberOf)
	}
	if len(role.AdminOf) != 1 || role.AdminOf[0] != "app_admins" {
		t.Fatalf("unexpected adminOf %#v", role.AdminOf)
	}
}

func TestFunctionDSLRegistersOptions(t *testing.T) {
	pg.Reset()
	t.Cleanup(pg.Reset)

	pg.FunctionInSchema("billing", "invoice_total", "integer", "SELECT 1").
		Args(
			pg.FunctionArg("invoice_id", "uuid"),
			pg.FunctionArg("scale", "integer").Default("1"),
		).
		Language("sql").
		Stable().
		Strict().
		SecurityDefiner().
		ParallelSafe().
		Cost(10).
		Rows(5).
		Set("search_path", "billing, public").
		Comment("Calculates an invoice total.").
		PreviousName("old_invoice_total")

	schema := pg.Schema()
	if len(schema.Functions) != 1 {
		t.Fatalf("functions len = %d, want 1", len(schema.Functions))
	}

	function := schema.Functions[0]
	if function.Schema != "billing" || function.Name != "invoice_total" || function.PreviousName != "old_invoice_total" {
		t.Fatalf("unexpected function identity %#v", function)
	}
	if function.Language != "sql" || function.ReturnType != "integer" || function.Body != "SELECT 1" {
		t.Fatalf("unexpected function core fields %#v", function)
	}
	if len(function.Arguments) != 2 {
		t.Fatalf("arguments len = %d, want 2", len(function.Arguments))
	}
	if function.Arguments[0].Name != "invoice_id" || function.Arguments[0].Type != "uuid" {
		t.Fatalf("unexpected first argument %#v", function.Arguments[0])
	}
	if function.Arguments[1].Default != "1" {
		t.Fatalf("unexpected second argument %#v", function.Arguments[1])
	}
	if function.Volatility != "STABLE" || function.Parallel != "SAFE" {
		t.Fatalf("unexpected function planner options %#v", function)
	}
	if function.Strict == nil || !*function.Strict {
		t.Fatalf("expected strict function, got %#v", function.Strict)
	}
	if !function.SecurityDefiner {
		t.Fatal("expected security definer")
	}
	if function.Cost == nil || *function.Cost != 10 {
		t.Fatalf("unexpected cost %#v", function.Cost)
	}
	if function.Rows == nil || *function.Rows != 5 {
		t.Fatalf("unexpected rows %#v", function.Rows)
	}
	if function.Configuration["search_path"] != "billing, public" {
		t.Fatalf("unexpected configuration %#v", function.Configuration)
	}
	if function.Comment != "Calculates an invoice total." {
		t.Fatalf("unexpected comment %q", function.Comment)
	}
}
