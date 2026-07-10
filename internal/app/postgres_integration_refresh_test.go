package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/render"
	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
)

func TestPostgresIntegrationRefreshMaterializedView(t *testing.T) {
	requireIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	schema := pgschema.Schema{
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "bookings",
				Columns: []ast.Column{
					{Name: "id", Type: "integer"},
					{Name: "owner", Type: "text"},
					{Name: "resource", Type: "text"},
				},
			},
		}},
		MaterializedViews: []pgschema.MaterializedView{{
			Name:   "cached_bookings",
			Query:  "SELECT owner, resource, count(*) AS booking_count FROM bookings GROUP BY owner, resource",
			NoData: true,
		}},
	}

	snapshot, snapshotID := integrationSnapshot(t, schema)

	dir := t.TempDir()
	baselineSQL, err := render.Postgres(schema)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationPlan(t, dir, goose.Renderer{}, migrate.Plan{
		Name:           "baseline",
		Dialect:        "postgresql",
		ToSnapshotID:   snapshotID,
		TargetSnapshot: snapshot,
		CreatedAt:      time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
		Changes:        []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpStatements:   []migrate.Statement{{SQL: baselineSQL}},
	})

	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    dir,
			Runner: DefaultMigrationsRunner,
		},
	}
	if _, err := MigrateRefreshWithConfig(config, MigrateRefreshOptions{
		View:      "cached_bookings",
		CreatedAt: time.Date(2026, 7, 8, 12, 1, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	migrations, err := migrate.ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 2 {
		t.Fatalf("expected baseline + refresh migrations, got %d", len(migrations))
	}

	dsn := newPostgresIntegrationDatabase(t, ctx)
	conn := openPostgres(t, ctx, dsn)
	defer func() {
		_ = conn.Close(context.Background())
	}()

	// Apply the baseline only: creates the bookings table and an unpopulated
	// (WITH NO DATA) cached_bookings materialized view.
	if _, err := pgtooling.Replay(ctx, conn, pgtooling.ReplayOptions{
		Runner:     DefaultMigrationsRunner,
		Migrations: migrations[:1],
	}); err != nil {
		t.Fatal(err)
	}

	// A WITH NO DATA materialized view is not queryable until it is refreshed.
	if err := conn.Exec(ctx, "SELECT count(*) FROM cached_bookings;"); err == nil {
		t.Fatal("expected querying an unpopulated materialized view to fail before refresh")
	} else if !strings.Contains(err.Error(), "has not been populated") {
		t.Fatalf("expected not-populated error before refresh, got %v", err)
	}

	execStatements(t, ctx, conn, []string{
		"INSERT INTO bookings (id, owner, resource) VALUES (1, 'alice', 'room-a'), (2, 'alice', 'room-a'), (3, 'bob', 'room-b');",
	})

	// Apply the generated refresh migration.
	if _, err := pgtooling.Replay(ctx, conn, pgtooling.ReplayOptions{
		Runner:     DefaultMigrationsRunner,
		Migrations: migrations[1:],
	}); err != nil {
		t.Fatal(err)
	}

	assertMaterializedViewBookingCounts(t, ctx, conn, map[string]int{"alice": 2, "bob": 1})
}

func assertMaterializedViewBookingCounts(t *testing.T, ctx context.Context, conn *pgtooling.Conn, want map[string]int) {
	t.Helper()
	rows, err := conn.Query(ctx, "SELECT owner, booking_count FROM cached_bookings ORDER BY owner;")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int{}
	for rows.Next() {
		var owner string
		var count int
		if err := rows.Scan(&owner, &count); err != nil {
			t.Fatal(err)
		}
		got[owner] = count
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("refreshed materialized view has %d owners, want %d: %v", len(got), len(want), got)
	}
	for owner, count := range want {
		if got[owner] != count {
			t.Fatalf("owner %q booking_count = %d, want %d (all: %v)", owner, got[owner], count, got)
		}
	}
}
