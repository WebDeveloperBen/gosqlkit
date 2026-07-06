package schema

import "github.com/webdeveloperben/gosqlkit/pg"

var PgCrypto = pg.Extension("pgcrypto")

var Billing = pg.Namespace("billing")

var InvoiceStatus = pg.EnumTypeInSchema("billing", "invoice_status", "draft", "issued", "paid", "void")

var OrderNumberSeq = pg.SequenceInSchema("billing", "order_number_seq", pg.SequenceOptions{
	Increment: 1,
	StartWith: ptrInt64(1000),
	Cache:     ptrInt64(1),
})

var Money = pg.CompositeTypeInSchema(
	"billing", "money",
	pg.CompositeAttribute("amount", "numeric(10, 2)"),
	pg.CompositeAttribute("currency", "char(3)"),
)

var Email = pg.Domain("email", "text").
	NotNull().
	Check("value ~ '^[^@]+@[^@]+$'")

var Users = pg.Table(
	"users",
	pg.UUID("id").
		PrimaryKey().
		Default("gen_random_uuid()"),
	pg.Text("email").
		NotNull(),
	pg.Text("display_name"),
	pg.Inet("last_login_ip"),
	pg.Text("tags").Array(),
	pg.TimestampTZ("created_at").
		NotNull().
		Default("now()"),
	pg.Unique("users_email_unique", "email"),
).Comment("Application users.")

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
		OnDelete("cascade").
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
		OnDelete("cascade"),
	pg.Check("invoice_lines_amount_cents_positive", "amount_cents > 0"),
	pg.IndexOn(
		"invoice_lines_description_idx",
		pg.IndexColumn("description").OpClass("text_ops"),
	).Using("btree").Concurrently().With("fillfactor", "90"),
)

var Events = pg.Table(
	"events",
	pg.Integer("id").GeneratedAlwaysAsIdentity(pg.SequenceOptions{
		Name:      "events_id_seq",
		Increment: 1,
		Cache:     ptrInt64(20),
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
	).Using("gist"),
)

func ptrInt64(v int64) *int64 {
	return &v
}
