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
	BTree   IndexMethod = "btree"
	Hash    IndexMethod = "hash"
	GiST    IndexMethod = "gist"
	SPGiST  IndexMethod = "spgist"
	GIN     IndexMethod = "gin"
	BRIN    IndexMethod = "brin"
	HNSW    IndexMethod = "hnsw"
	IVFFlat IndexMethod = "ivfflat"
)

func CustomIndexMethod(method string) IndexMethod {
	if strings.TrimSpace(method) == "" {
		panic("custom index method must not be empty")
	}
	return IndexMethod(method)
}

type IndexOpClass string

const (
	VectorL2Ops     IndexOpClass = "vector_l2_ops"
	VectorIPOps     IndexOpClass = "vector_ip_ops"
	VectorCosineOps IndexOpClass = "vector_cosine_ops"
	VectorL1Ops     IndexOpClass = "vector_l1_ops"

	HalfVecL2Ops     IndexOpClass = "halfvec_l2_ops"
	HalfVecIPOps     IndexOpClass = "halfvec_ip_ops"
	HalfVecCosineOps IndexOpClass = "halfvec_cosine_ops"
	HalfVecL1Ops     IndexOpClass = "halfvec_l1_ops"

	SparseVecL2Ops     IndexOpClass = "sparsevec_l2_ops"
	SparseVecIPOps     IndexOpClass = "sparsevec_ip_ops"
	SparseVecCosineOps IndexOpClass = "sparsevec_cosine_ops"
	SparseVecL1Ops     IndexOpClass = "sparsevec_l1_ops"

	BitHammingOps IndexOpClass = "bit_hamming_ops"
	BitJaccardOps IndexOpClass = "bit_jaccard_ops"
)

func CustomIndexOpClass(opClass string) IndexOpClass {
	if strings.TrimSpace(opClass) == "" {
		panic("custom index operator class must not be empty")
	}
	return IndexOpClass(opClass)
}

type ViewCheckOption string

const (
	LocalCheckOption    ViewCheckOption = "LOCAL"
	CascadedCheckOption ViewCheckOption = "CASCADED"
)
