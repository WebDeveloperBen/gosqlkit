package pg

import "strings"

type ForeignKeyAction string

const (
	NoAction   ForeignKeyAction = "no action"
	Restrict   ForeignKeyAction = "restrict"
	Cascade    ForeignKeyAction = "cascade"
	SetNull    ForeignKeyAction = "set null"
	SetDefault ForeignKeyAction = "set default"
)

type IndexMethod string

const (
	BTree  IndexMethod = "btree"
	Hash   IndexMethod = "hash"
	GiST   IndexMethod = "gist"
	SPGiST IndexMethod = "spgist"
	GIN    IndexMethod = "gin"
	BRIN   IndexMethod = "brin"
)

func CustomIndexMethod(method string) IndexMethod {
	if strings.TrimSpace(method) == "" {
		panic("custom index method must not be empty")
	}
	return IndexMethod(method)
}

type ViewCheckOption string

const (
	LocalCheckOption    ViewCheckOption = "LOCAL"
	CascadedCheckOption ViewCheckOption = "CASCADED"
)
