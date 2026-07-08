package tooling

import (
	"strings"
	"testing"
)

func TestSplitSQLStatementsPreservesDollarQuotedBodies(t *testing.T) {
	got := SplitSQLStatements("CREATE FUNCTION f() RETURNS trigger LANGUAGE plpgsql AS $fn$\nBEGIN\nRETURN NEW;\nEND\n$fn$;\nCREATE INDEX CONCURRENTLY users_email_idx ON users (email);")
	if len(got) != 2 {
		t.Fatalf("statements = %#v", got)
	}
	if !strings.Contains(got[0], "RETURN NEW;") || !strings.Contains(got[1], "CONCURRENTLY") {
		t.Fatalf("statements = %#v", got)
	}
}

func TestSplitSQLStatementsPreservesQuotedSemicolons(t *testing.T) {
	got := SplitSQLStatements(`INSERT INTO logs(message) VALUES ('don''t split; here');
CREATE TABLE "semi;colon" (id int);
-- comment; still not a statement
/* block; comment */
SELECT 1;`)
	if len(got) != 3 {
		t.Fatalf("statements = %#v", got)
	}
	for _, statement := range got {
		if !strings.HasSuffix(statement, ";") {
			t.Fatalf("statement missing terminator: %q", statement)
		}
	}
}
