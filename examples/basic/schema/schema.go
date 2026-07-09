package schema

import "github.com/webdeveloperben/gosqlkit/pg"

var PgCrypto = pg.Extension("pgcrypto")

var AppReader = pg.Role("app_reader").NoLogin()

var AppWriter = pg.Role("app_writer").
	Login().
	MemberOf("app_reader").
	ConnectionLimit(20)

var Billing = pg.Namespace("billing")

var InvoiceStatus = pg.EnumTypeInSchema("billing", "invoice_status", "draft", "issued", "paid", "void")

var OrderNumberSeq = pg.SequenceInSchema("billing", "order_number_seq", pg.SequenceOptions{
	Increment: 1,
	StartWith: new(int64(1000)),
	Cache:     new(int64(1)),
})

var NormaliseEmail = pg.SQLFunction("normalise_email").
	Returns(pg.TextType()).
	Args(pg.FunctionArg("email", pg.TextType())).
	Body(pg.Select(pg.Lower(pg.Trim(pg.Col("email"))))).
	Immutable().
	Strict()

var SetUpdatedAt = pg.PLpgSQLFunction("set_updated_at").
	Returns(pg.TriggerType()).
	Body(pg.Block(
		pg.Assign("NEW.updated_at", pg.Call("now")),
		pg.Return(pg.Col("NEW")),
	)).
	Volatile()

var Money = pg.CompositeTypeInSchema(
	"billing", "money",
	pg.CompositeField(pg.Numeric("amount", 10, 2)),
	pg.CompositeField(pg.Char("currency", 3)),
)

var Email = pg.Domain("email", "text").
	NotNull().
	Check("value ~ '^[^@]+@[^@]+$'")

var Users = pg.Table(
	"users",
	pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"),
	pg.Text("email").NotNull(),
	pg.Text("display_name"),
	pg.Inet("last_login_ip"),
	pg.Text("tags").Array(),
	pg.TimestampTZ("created_at").NotNull().Default("now()"),
	pg.TimestampTZ("updated_at").NotNull().Default("now()"),
	pg.Unique("users_email_unique", "email"),
).
	Comment("Application users.").
	EnableRLS()

var UsersSetUpdatedAt = pg.Trigger("users_set_updated_at", "users", "set_updated_at").
	Before().
	UpdateOf("email", "display_name").
	ForEachRow().
	When("OLD.* IS DISTINCT FROM NEW.*").
	Comment("Automatically updates updated_at timestamp on row updates.")

var UsersReadSelfPolicy = pg.Policy("users_read_self", "users").
	Select().
	To("app_reader").
	Using("id = current_setting('app.user_id')::uuid")

var Invoices = pg.TableInSchema(
	"billing", "invoices",
	pg.UUID("id").
		PrimaryKey().
		Default("gen_random_uuid()"),
	pg.UUID("user_id").
		NotNull(),
	pg.Integer("amount_cents").
		NotNull(),
	pg.EnumColumn("status", InvoiceStatus).
		NotNull().
		Default("'draft'"),
	pg.TimestampTZ("created_at").
		NotNull().
		Default("now()"),
	pg.Check("invoices_amount_cents_positive", "amount_cents > 0"),
	pg.ForeignKey("invoices_user_id_fkey", "user_id").
		References("public.users", "id").
		OnDelete(pg.Cascade).
		InitiallyDeferred(),
	pg.IndexOn(
		"invoices_user_id_created_at_idx",
		pg.IndexColumn("user_id"),
		pg.IndexColumn("created_at").Desc().NullsLast(),
	).Where("status <> 'void'"),
)

var InvoiceLines = pg.TableInSchema(
	"billing", "invoice_lines",
	pg.UUID("invoice_id").NotNull(),
	pg.Integer("line_no").NotNull(),
	pg.Text("description").NotNull(),
	pg.Integer("amount_cents").NotNull(),
	pg.PrimaryKey("invoice_lines_pkey", "invoice_id", "line_no"),
	pg.ForeignKey("invoice_lines_invoice_id_fkey", "invoice_id").
		References("billing.invoices", "id").
		OnDelete(pg.Cascade),
	pg.Check("invoice_lines_amount_cents_positive", "amount_cents > 0"),
	pg.IndexOn(
		"invoice_lines_description_idx",
		pg.IndexColumn("description").OpClass("text_ops"),
	).Using(pg.BTree).Concurrently().With("fillfactor", "90"),
)

var Events = pg.Table(
	"events",
	pg.Integer("id").GeneratedAlwaysAsIdentity(pg.SequenceOptions{
		Name:      "events_id_seq",
		Increment: 1,
		Cache:     new(int64(20)),
	}),
	pg.Text("description").NotNull(),
	pg.Text("search_vector").GeneratedAlwaysAs("to_tsvector('english', description)"),
	pg.JSONB("metadata").DefaultJSON(map[string]any{"source": "api"}),
	pg.Bytea("payload"),
	pg.Interval("duration", pg.IntervalConfig{Fields: "day to second", Precision: 6}).NotNull(),
	pg.SmallInt("priority").DefaultInt(0).NotNull(),
	pg.TimestampTZ("created_at").
		NotNull().
		Default("now()"),
	pg.Index("", "priority", "created_at"),
	pg.PartitionByRange("priority"),
)

var EventsPriorityLow = pg.Table(
	"events_priority_low",
	pg.PartitionOf("events", pg.ForValuesFromTo(
		pg.PartitionValues("0"),
		pg.PartitionValues("10"),
	)),
)

var EventsDefault = pg.Table(
	"events_default",
	pg.PartitionOf("events", pg.ForValuesDefault()),
)

var Bookings = pg.Table(
	"bookings",
	pg.UUID("id").
		PrimaryKey().
		Default("gen_random_uuid()"),
	pg.Text("resource").NotNull(),
	pg.Text("owner").NotNull(),
	pg.Custom("during", "tstzrange").NotNull(),
	pg.Exclusion(
		"bookings_no_overlap",
		pg.ExcludeWith("during", "&&"),
	).Using(pg.GiST),
)

var ActiveUsers = pg.View("active_users").
	As(pg.Select(
		pg.Col("id"),
		pg.Col("email"),
		pg.Col("display_name"),
	).From("users").Where(pg.IsNotNull(pg.Col("last_login_ip")))).
	Comment("Users who have logged in at least once.")

var UserInvoiceSummary = pg.ViewInSchema(
	"billing", "user_invoice_summary",
).As(pg.Select(
	pg.Col("user_id"),
	pg.CountAll().As("invoice_count"),
	pg.Sum(pg.Col("amount_cents")).As("total_cents"),
).From("billing.invoices").GroupBy(pg.Col("user_id"))).
	Columns("user_id", "invoice_count", "total_cents").
	DependsOn("billing.invoices")

var CachedBookings = pg.MaterializedView("cached_bookings").
	As(pg.Select(
		pg.Col("owner"),
		pg.Col("resource"),
		pg.CountAll().As("booking_count"),
	).From("bookings").GroupBy(pg.Col("owner"), pg.Col("resource"))).
	Comment("Pre-aggregated booking counts.").
	DependsOn("bookings")

var ReaderUserGrant = pg.Grant(pg.PrivilegeSelect).
	OnTable("users").
	To("app_reader")

var WriterUserGrant = pg.Grant(pg.PrivilegeInsert, pg.PrivilegeUpdate).
	Columns(pg.PrivilegeReferences, "id").
	OnTable("users").
	To("app_writer")

var BillingSchemaGrant = pg.Grant(pg.PrivilegeUsage).
	OnSchema("billing").
	To("app_reader")

var OrderNumberGrant = pg.Grant(pg.PrivilegeUsage, pg.PrivilegeSelect).
	OnSequence("billing.order_number_seq").
	To("app_writer")

var InvoiceStatusGrant = pg.Grant(pg.PrivilegeUsage).
	OnType("billing.invoice_status").
	To("app_reader")

var NormaliseEmailGrant = pg.Grant(pg.PrivilegeExecute).
	OnFunction("normalise_email(email text)").
	To("app_writer")
