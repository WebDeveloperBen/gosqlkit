package schema

import "github.com/webdeveloperben/pgkit/pg"

var Users = pg.Table("users",
	pg.UUID("id").
		PrimaryKey().
		Default("gen_random_uuid()"),
	pg.Text("email").
		NotNull().
		Unique(),
	pg.Text("display_name"),
	pg.TimestampTZ("created_at").
		NotNull().
		Default("now()"),
)

var Invoices = pg.Table("invoices",
	pg.UUID("id").
		PrimaryKey().
		Default("gen_random_uuid()"),
	pg.UUID("user_id").
		NotNull().
		References("users", "id").
		OnDelete("cascade"),
	pg.Integer("amount_cents").
		NotNull(),
	pg.Text("status").
		NotNull().
		Default("'draft'"),
	pg.TimestampTZ("created_at").
		NotNull().
		Default("now()"),
	pg.Check("invoices_amount_cents_positive", "amount_cents > 0"),
	pg.Index("invoices_user_id_idx", "user_id"),
)
