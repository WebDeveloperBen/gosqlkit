package sqlite

// ForeignKeyAction is a typed SQLite foreign-key referential action.
type ForeignKeyAction string

const (
	NoAction   ForeignKeyAction = "no action"
	Restrict   ForeignKeyAction = "restrict"
	Cascade    ForeignKeyAction = "cascade"
	SetNull    ForeignKeyAction = "set null"
	SetDefault ForeignKeyAction = "set default"
)

// Built-in SQLite collation sequences. Application-registered collations can be
// passed to Collate as plain strings.
const (
	Binary = "BINARY"
	NoCase = "NOCASE"
	RTrim  = "RTRIM"
)
