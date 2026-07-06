package schema

import "github.com/webdeveloperben/gosqlkit/pg"

var PgCrypto = pg.Extension("pgcrypto")

var Billing = pg.Namespace("billing")

var InvoiceStatus = pg.EnumTypeInSchema("billing", "invoice_status", "draft", "issued", "paid", "void")

var Users = pg.Table(
	"users",
	pg.UUID("id").
		PrimaryKey().
		Default("gen_random_uuid()"),
	pg.Text("email").
		NotNull(),
	pg.Text("display_name"),
	pg.TimestampTZ("created_at").
		NotNull().
		Default("now()"),
	pg.Unique("users_email_unique", "email"),
)

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
		OnDelete("cascade"),
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
