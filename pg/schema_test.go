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

func TestGrantDSLRegistersOptions(t *testing.T) {
	pg.Reset()
	t.Cleanup(pg.Reset)

	pg.Grant(pg.PrivilegeSelect).
		Columns(pg.PrivilegeUpdate, "display_name").
		OnTable("users").
		To("app_reader").
		WithGrantOption()

	schema := pg.Schema()
	if len(schema.Grants) != 1 {
		t.Fatalf("grants len = %d, want 1", len(schema.Grants))
	}
	grant := schema.Grants[0]
	if grant.Target.Type != "table" || grant.Target.Name != "users" {
		t.Fatalf("target = %#v", grant.Target)
	}
	if len(grant.Privileges) != 2 || grant.Privileges[0].Name != "SELECT" || grant.Privileges[1].Name != "UPDATE" {
		t.Fatalf("privileges = %#v", grant.Privileges)
	}
	if len(grant.Privileges[1].Columns) != 1 || grant.Privileges[1].Columns[0] != "display_name" {
		t.Fatalf("column privileges = %#v", grant.Privileges[1].Columns)
	}
	if len(grant.Grantees) != 1 || grant.Grantees[0] != "app_reader" || !grant.GrantOption {
		t.Fatalf("grant options = %#v", grant)
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

func TestFunctionBuilderDSLRegistersOptions(t *testing.T) {
	pg.Reset()
	t.Cleanup(pg.Reset)

	pg.SQLFunctionInSchema("billing", "normalise_email").
		Returns(pg.TextType()).
		Args(pg.FunctionArg("email", pg.TextType())).
		Body(pg.Select(pg.Lower(pg.Trim(pg.Col("email"))))).
		Immutable().
		Strict()

	pg.PLpgSQLFunction("touch_user").
		Returns(pg.TriggerType()).
		Body(pg.Block(
			pg.Assign("NEW.updated_at", pg.Call("now")),
			pg.Return(pg.Col("NEW")),
		))

	schema := pg.Schema()
	if len(schema.Functions) != 2 {
		t.Fatalf("functions len = %d, want 2", len(schema.Functions))
	}

	sqlFunction := schema.Functions[0]
	if sqlFunction.Schema != "billing" || sqlFunction.Name != "normalise_email" ||
		sqlFunction.Language != "sql" || sqlFunction.ReturnType != "text" ||
		sqlFunction.Body != "SELECT lower(trim(email))" {
		t.Fatalf("unexpected SQL function %#v", sqlFunction)
	}
	if len(sqlFunction.Arguments) != 1 || sqlFunction.Arguments[0].Type != "text" {
		t.Fatalf("unexpected SQL function args %#v", sqlFunction.Arguments)
	}

	triggerFunction := schema.Functions[1]
	if triggerFunction.Language != "plpgsql" || triggerFunction.ReturnType != "trigger" ||
		triggerFunction.Body != "BEGIN\n    NEW.updated_at = now();\n    RETURN NEW;\nEND" {
		t.Fatalf("unexpected PLpgSQL function %#v", triggerFunction)
	}
}

func TestTriggerDSLRegistersOptions(t *testing.T) {
	pg.Reset()
	t.Cleanup(pg.Reset)

	pg.Trigger("users_touch", "users", "touch_user").
		Before().
		UpdateOf("email", "display_name").
		ForEachRow().
		When("OLD.* IS DISTINCT FROM NEW.*").
		Args("updated_at").
		PreviousName("old_users_touch")

	schema := pg.Schema()
	if len(schema.Triggers) != 1 {
		t.Fatalf("triggers len = %d, want 1", len(schema.Triggers))
	}

	trigger := schema.Triggers[0]
	if trigger.Name != "users_touch" || trigger.PreviousName != "old_users_touch" {
		t.Fatalf("unexpected trigger identity %#v", trigger)
	}
	if trigger.Target != "users" || trigger.Function != "touch_user" {
		t.Fatalf("unexpected trigger target/function %#v", trigger)
	}
	if trigger.Timing != "BEFORE" || trigger.Level != "ROW" {
		t.Fatalf("unexpected trigger timing/level %#v", trigger)
	}
	if len(trigger.Events) != 1 || trigger.Events[0] != "UPDATE" {
		t.Fatalf("unexpected trigger events %#v", trigger.Events)
	}
	if len(trigger.Columns) != 2 || trigger.Columns[0] != "email" || trigger.Columns[1] != "display_name" {
		t.Fatalf("unexpected trigger columns %#v", trigger.Columns)
	}
	if trigger.When != "OLD.* IS DISTINCT FROM NEW.*" {
		t.Fatalf("unexpected trigger when %q", trigger.When)
	}
	if len(trigger.Arguments) != 1 || trigger.Arguments[0] != "updated_at" {
		t.Fatalf("unexpected trigger arguments %#v", trigger.Arguments)
	}
}

func TestViewBuilderDSLRegistersQueries(t *testing.T) {
	pg.Reset()
	t.Cleanup(pg.Reset)

	pg.View("active_users").
		As(pg.Select(
			pg.Col("id"),
			pg.Col("email"),
			pg.Col("display_name"),
		).From("users").Where(pg.IsNotNull(pg.Col("last_login_ip"))))

	pg.MaterializedView("booking_counts").
		As(pg.Select(
			pg.Col("owner"),
			pg.CountAll().As("booking_count"),
		).From("bookings").GroupBy(pg.Col("owner")))

	schema := pg.Schema()
	if len(schema.Views) != 1 || schema.Views[0].Query != "SELECT id, email, display_name FROM users WHERE last_login_ip IS NOT NULL" {
		t.Fatalf("unexpected view %#v", schema.Views)
	}
	if len(schema.MaterializedViews) != 1 || schema.MaterializedViews[0].Query != "SELECT owner, COUNT(*) AS booking_count FROM bookings GROUP BY owner" {
		t.Fatalf("unexpected materialized view %#v", schema.MaterializedViews)
	}
}

func TestTypedPostgresOptions(t *testing.T) {
	pg.Reset()
	t.Cleanup(pg.Reset)

	pg.Table(
		"invoices",
		pg.UUID("user_id").References("users", "id").OnDelete(pg.Cascade).OnUpdate(pg.SetNull),
		pg.ForeignKey("invoices_user_id_fkey", "user_id").
			References("users", "id").
			OnDelete(pg.Restrict).
			OnUpdate(pg.NoAction),
		pg.IndexOn("invoices_user_id_idx", pg.IndexColumn("user_id")).Using(pg.BTree),
	)
	pg.View("checked_users", "SELECT id FROM users").CheckOption(pg.LocalCheckOption)

	schema := pg.Schema()
	table := schema.Tables[0]
	if table.Columns[0].References.OnDelete != "cascade" || table.Columns[0].References.OnUpdate != "set null" {
		t.Fatalf("unexpected column FK actions %#v", table.Columns[0].References)
	}
	if table.ForeignKeys[0].OnDelete != "restrict" || table.ForeignKeys[0].OnUpdate != "no action" {
		t.Fatalf("unexpected table FK actions %#v", table.ForeignKeys[0])
	}
	if table.Indexes[0].Method != "btree" {
		t.Fatalf("unexpected index method %#v", table.Indexes[0])
	}
	if schema.Views[0].CheckOption != "LOCAL" {
		t.Fatalf("unexpected view check option %#v", schema.Views[0])
	}
}

func TestTablePartitionDSLRegistersOptions(t *testing.T) {
	pg.Reset()
	t.Cleanup(pg.Reset)

	pg.Table(
		"events",
		pg.TimestampTZ("created_at").NotNull(),
		pg.Text("description"),
		pg.PartitionByRange("created_at"),
	)
	pg.Table(
		"audit_events",
		pg.Integer("tenant_id").NotNull(),
		pg.Text("description"),
	).PartitionByHash("tenant_id")
	pg.Table(
		"audit_events_0",
		pg.PartitionOf("audit_events", pg.ForValuesWith(4, 0)),
	)
	pg.Table(
		"email_events",
		pg.Text("email").NotNull(),
		pg.PartitionByHashOn(pg.PartitionExpression("lower(email)")),
	)

	schema := pg.Schema()
	if len(schema.Tables) != 4 {
		t.Fatalf("tables len = %d, want 4", len(schema.Tables))
	}
	events := schema.Tables[0]
	if events.Partitioning == nil || events.Partitioning.Strategy != "range" || len(events.Partitioning.Keys) != 1 || events.Partitioning.Keys[0].Expression != "created_at" {
		t.Fatalf("unexpected events partitioning %#v", events.Partitioning)
	}
	auditEvents := schema.Tables[1]
	if auditEvents.Partitioning == nil || auditEvents.Partitioning.Strategy != "hash" || len(auditEvents.Partitioning.Keys) != 1 || auditEvents.Partitioning.Keys[0].Expression != "tenant_id" {
		t.Fatalf("unexpected audit_events partitioning %#v", auditEvents.Partitioning)
	}
	partition := schema.Tables[2]
	if partition.PartitionOf == nil || partition.PartitionOf.Parent != "audit_events" ||
		partition.PartitionOf.Bound.Type != "hash" || partition.PartitionOf.Bound.Modulus != 4 ||
		partition.PartitionOf.Bound.Remainder != 0 {
		t.Fatalf("unexpected audit_events_0 partition %#v", partition.PartitionOf)
	}
	emailEvents := schema.Tables[3]
	if emailEvents.Partitioning == nil || emailEvents.Partitioning.Strategy != "hash" ||
		len(emailEvents.Partitioning.Keys) != 1 || emailEvents.Partitioning.Keys[0].Expression != "lower(email)" ||
		!emailEvents.Partitioning.Keys[0].IsExpression {
		t.Fatalf("unexpected email_events partitioning %#v", emailEvents.Partitioning)
	}
}

func TestPolicyDSLRegistersOptions(t *testing.T) {
	pg.Reset()
	t.Cleanup(pg.Reset)

	pg.Table("users", pg.UUID("id")).
		EnableRLS()

	pg.Policy("users_read_self", "users").
		Permissive().
		Select().
		To("app_reader").
		Using("id = current_setting('app.user_id')::uuid").
		PreviousName("old_users_read_self")

	schema := pg.Schema()
	if len(schema.Tables) != 1 {
		t.Fatalf("tables len = %d, want 1", len(schema.Tables))
	}
	if !schema.Tables[0].RowLevelSecurity || schema.Tables[0].ForceRLS {
		t.Fatalf("unexpected table RLS flags %#v", schema.Tables[0])
	}
	if len(schema.Policies) != 1 {
		t.Fatalf("policies len = %d, want 1", len(schema.Policies))
	}

	policy := schema.Policies[0]
	if policy.Name != "users_read_self" || policy.PreviousName != "old_users_read_self" {
		t.Fatalf("unexpected policy identity %#v", policy)
	}
	if policy.Table != "users" || policy.Command != "SELECT" || policy.Mode != "PERMISSIVE" {
		t.Fatalf("unexpected policy core fields %#v", policy)
	}
	if len(policy.Roles) != 1 || policy.Roles[0] != "app_reader" {
		t.Fatalf("unexpected policy roles %#v", policy.Roles)
	}
	if policy.Using != "id = current_setting('app.user_id')::uuid" {
		t.Fatalf("unexpected USING expression %q", policy.Using)
	}
}

func TestTableDSLForceRLSEnablesRLS(t *testing.T) {
	pg.Reset()
	t.Cleanup(pg.Reset)

	pg.Table("users", pg.UUID("id")).
		ForceRLS()

	schema := pg.Schema()
	if len(schema.Tables) != 1 {
		t.Fatalf("tables len = %d, want 1", len(schema.Tables))
	}
	if !schema.Tables[0].RowLevelSecurity || !schema.Tables[0].ForceRLS {
		t.Fatalf("unexpected table RLS flags %#v", schema.Tables[0])
	}
}
