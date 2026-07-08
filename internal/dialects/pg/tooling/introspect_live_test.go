package tooling_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
)

func TestIntrospectLivePostgres(t *testing.T) {
	dsn := os.Getenv("GOSQLKIT_PG_TEST_DSN")
	if dsn == "" {
		t.Skip("set GOSQLKIT_PG_TEST_DSN to run live PostgreSQL introspection test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgtooling.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	sql, err := os.ReadFile("../../../../examples/basic/db/schema.generated.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range splitSQLStatements(string(sql)) {
		if err := conn.Exec(ctx, statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}

	schema, err := pgtooling.Introspect(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Roles) < 2 {
		t.Fatalf("roles = %#v", schema.Roles)
	}
	if len(schema.Functions) < 2 {
		t.Fatalf("functions = %#v", schema.Functions)
	}
	if len(schema.Views) < 2 {
		t.Fatalf("views = %#v", schema.Views)
	}
	if len(schema.MaterializedViews) < 1 {
		t.Fatalf("materialized views = %#v", schema.MaterializedViews)
	}
	if len(schema.Triggers) < 1 {
		t.Fatalf("triggers = %#v", schema.Triggers)
	}
	if len(schema.Policies) < 1 {
		t.Fatalf("policies = %#v", schema.Policies)
	}
}

func splitSQLStatements(sql string) []string {
	var out []string
	var b strings.Builder
	inString := false
	inDollar := false
	for i := 0; i < len(sql); i++ {
		if !inString && i+1 < len(sql) && sql[i:i+2] == "$$" {
			inDollar = !inDollar
			b.WriteString("$$")
			i++
			continue
		}
		switch sql[i] {
		case '\'':
			if !inDollar {
				inString = !inString
			}
		case ';':
			if !inString && !inDollar {
				statement := strings.TrimSpace(b.String())
				if statement != "" {
					out = append(out, statement+";")
				}
				b.Reset()
				continue
			}
		}
		b.WriteByte(sql[i])
	}
	if statement := strings.TrimSpace(b.String()); statement != "" {
		out = append(out, statement)
	}
	return out
}
