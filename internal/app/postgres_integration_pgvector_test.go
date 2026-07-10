package app

import (
	"context"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/render"
	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
)

func TestPostgresIntegrationPgVectorTypes(t *testing.T) {
	requireIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	dsn := newPostgresIntegrationDatabase(t, ctx)
	conn := openPostgres(t, ctx, dsn)
	defer func() {
		_ = conn.Close(context.Background())
	}()

	version, comment, ok := availableExtensionInfo(t, ctx, conn, "vector")
	if !ok {
		t.Skip("selected PostgreSQL image does not provide the vector extension")
	}

	schema := pgschema.Schema{
		Extensions: []pgschema.Extension{
			{Name: "vector", Version: version, Comment: comment},
		},
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "documents",
				Columns: []ast.Column{
					{Name: "id", Type: "integer", PrimaryKey: true},
					{Name: "embedding", Type: "vector(3)", NotNull: true},
					{Name: "summary_embedding", Type: "halfvec(3)", NotNull: true},
					{Name: "sparse_embedding", Type: "sparsevec(3)", NotNull: true},
					{Name: "binary_embedding", Type: "bit(3)", NotNull: true},
				},
			},
			Indexes: []pgschema.Index{
				{
					Index:  ast.Index{Name: "documents_embedding_hnsw_idx"},
					Method: "hnsw",
					Columns: []pgschema.IndexColumn{
						{IndexColumn: ast.IndexColumn{Expression: "embedding"}, OpClass: "vector_cosine_ops"},
					},
					With: map[string]string{"ef_construction": "64", "m": "16"},
				},
				{
					Index:  ast.Index{Name: "documents_embedding_ivfflat_idx"},
					Method: "ivfflat",
					Columns: []pgschema.IndexColumn{
						{IndexColumn: ast.IndexColumn{Expression: "embedding"}, OpClass: "vector_l2_ops"},
					},
					With: map[string]string{"lists": "1"},
				},
				{
					Index:  ast.Index{Name: "documents_summary_embedding_hnsw_idx"},
					Method: "hnsw",
					Columns: []pgschema.IndexColumn{
						{IndexColumn: ast.IndexColumn{Expression: "summary_embedding"}, OpClass: "halfvec_l2_ops"},
					},
				},
				{
					Index:  ast.Index{Name: "documents_sparse_embedding_hnsw_idx"},
					Method: "hnsw",
					Columns: []pgschema.IndexColumn{
						{IndexColumn: ast.IndexColumn{Expression: "sparse_embedding"}, OpClass: "sparsevec_l2_ops"},
					},
				},
				{
					Index:  ast.Index{Name: "documents_binary_embedding_hnsw_idx"},
					Method: "hnsw",
					Columns: []pgschema.IndexColumn{
						{IndexColumn: ast.IndexColumn{Expression: "binary_embedding"}, OpClass: "bit_jaccard_ops"},
					},
				},
			},
		}},
	}
	snapshot, _ := integrationSnapshot(t, schema)
	sql, err := render.Postgres(schema)
	if err != nil {
		t.Fatal(err)
	}
	execSQLStatements(t, ctx, conn, sql)

	execStatements(t, ctx, conn, []string{
		"INSERT INTO documents (id, embedding, summary_embedding, sparse_embedding, binary_embedding) VALUES (1, '[1,2,3]'::vector(3), '[1,2,3]'::halfvec(3), '{1:1,3:2}/3'::sparsevec(3), B'101'::bit(3));",
	})
	assertPgVectorRowCount(t, ctx, conn, 1)
	assertPgVectorColumnsIntrospected(t, ctx, conn)
	assertDatabaseMatchesSnapshot(t, ctx, conn, snapshot)
}

func availableExtensionInfo(t *testing.T, ctx context.Context, conn *pgtooling.Conn, name string) (string, string, bool) {
	t.Helper()
	rows, err := conn.Query(ctx, "SELECT default_version, comment FROM pg_available_extensions WHERE name = $1;", name)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return "", "", false
	}
	var version, comment string
	if err := rows.Scan(&version, &comment); err != nil {
		t.Fatal(err)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return version, comment, true
}

func assertPgVectorRowCount(t *testing.T, ctx context.Context, conn *pgtooling.Conn, want int) {
	t.Helper()
	rows, err := conn.Query(ctx, "SELECT count(*) FROM documents;")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("documents count query returned no rows")
	}
	var got int
	if err := rows.Scan(&got); err != nil {
		t.Fatal(err)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("documents count = %d, want %d", got, want)
	}
}

func assertPgVectorColumnsIntrospected(t *testing.T, ctx context.Context, conn *pgtooling.Conn) {
	t.Helper()
	schema, err := pgtooling.Introspect(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Extensions) != 1 || schema.Extensions[0].Name != "vector" || schema.Extensions[0].Version == "" {
		t.Fatalf("vector extension was not introspected: %#v", schema.Extensions)
	}
	if len(schema.Tables) != 1 || schema.Tables[0].Name != "documents" {
		t.Fatalf("documents table was not introspected: %#v", schema.Tables)
	}
	if len(schema.Enums) != 0 || len(schema.CompositeTypes) != 0 || len(schema.Domains) != 0 {
		t.Fatalf("extension-owned type objects leaked into schema: %#v", schema)
	}
	want := map[string]string{
		"id":                "integer",
		"embedding":         "vector(3)",
		"summary_embedding": "halfvec(3)",
		"sparse_embedding":  "sparsevec(3)",
		"binary_embedding":  "bit(3)",
	}
	for _, column := range schema.Tables[0].Columns {
		if want[column.Name] == "" {
			t.Fatalf("unexpected column introspected: %#v", column)
		}
		if column.Type != want[column.Name] {
			t.Fatalf("column %s type = %q, want %q", column.Name, column.Type, want[column.Name])
		}
		delete(want, column.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing pgvector columns: %#v", want)
	}

	wantIndexes := map[string]struct {
		method  string
		opClass string
	}{
		"documents_embedding_hnsw_idx":         {method: "hnsw", opClass: "vector_cosine_ops"},
		"documents_embedding_ivfflat_idx":      {method: "ivfflat", opClass: "vector_l2_ops"},
		"documents_summary_embedding_hnsw_idx": {method: "hnsw", opClass: "halfvec_l2_ops"},
		"documents_sparse_embedding_hnsw_idx":  {method: "hnsw", opClass: "sparsevec_l2_ops"},
		"documents_binary_embedding_hnsw_idx":  {method: "hnsw", opClass: "bit_jaccard_ops"},
	}
	for _, index := range schema.Tables[0].Indexes {
		want, ok := wantIndexes[index.Name]
		if !ok {
			t.Fatalf("unexpected index introspected: %#v", index)
		}
		if index.Method != want.method || len(index.Columns) != 1 || index.Columns[0].OpClass != want.opClass {
			t.Fatalf("index %s = %#v, want method %q opclass %q", index.Name, index, want.method, want.opClass)
		}
		delete(wantIndexes, index.Name)
	}
	if len(wantIndexes) != 0 {
		t.Fatalf("missing pgvector indexes: %#v", wantIndexes)
	}
}
