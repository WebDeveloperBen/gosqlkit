# gosqlkit SQLite Dialect Roadmap

This document tracks the **SQLite** dialect build against the goal of full
feature parity with the PostgreSQL dialect (see [FEATURES.md](FEATURES.md) for
the PostgreSQL board). It is the live status board for SQLite work: flip
checkboxes as things land. It is the SQLite counterpart to FEATURES.md and the
source of truth for "what's done / what's next" for SQLite.

SQLite is deliberately a **subset** of what PostgreSQL models. Many PostgreSQL
objects have no SQLite equivalent and are marked `[no]` with the idiomatic
SQLite alternative noted. This is not missing work — it is the shape of the
dialect boundary, and confirming it is exactly what proves `internal/ast` /
`kit` are genuinely dialect-neutral (FEATURES.md "First non-PostgreSQL provider
proving the boundary").

Read first, treat as canonical:

- [AGENTS.md](AGENTS.md) — architecture, layering, and the "Adding a new
  dialect" checklist (section 3).
- [FEATURES.md](FEATURES.md) — the PostgreSQL board this mirrors.
- [SPEC.md](SPEC.md) — goals, non-goals, scope.
- [MIGRATIONS.md](MIGRATIONS.md) — migration-generation decision record.

Legend:

- `[x]` implemented
- `[~]` partially implemented
- `[ ]` not implemented (planned for full parity)
- `[later]` intentionally later scope
- `[no]` not supported by SQLite / non-goal — with the idiomatic alternative

Reference implementations (already cloned locally while researching):

- Drizzle SQLite: `/tmp/gosqlkit-drizzle-ref/drizzle-orm/src/sqlite-core/`,
  `drizzle-kit/src/serializer/sqliteSerializer.ts`, `sqlgenerator.ts`.
- Atlas SQLite: `/tmp/gosqlkit-atlas-ref/sql/sqlite/`
  (`sqlite.go`, `diff.go`, `migrate.go`, `inspect.go`, `convert.go`).

## Status

- **Landed (build order steps 1–5): schema generation + migration authoring.**
  The public `sqlite/` DSL, `internal/dialects/sqlite/{sqliteschema,render,plan}`,
  kit provider registration, `examples/sqlite/` with checked-in goldens, Taskfile
  wiring, and the migration planner (all slices incl. the table-rebuild) are in
  place and green. `gosqlkit generate`/`snapshot`/`migrate create`/`migrate plan`
  work for SQLite. Generated DDL is validated against a real `sqlite3` binary.
  The generic diff helpers (`SortChangesByDependencies`, `MapBy`, `SortedBy`)
  were lifted into the shared `internal/migrate/plan` and PostgreSQL refactored
  onto them (refactor-then-reuse; PG's suite stayed green).
- **Remaining (build order steps 6–7):** tooling (`tooling/`: PRAGMA
  introspection, drift, sandbox replay, `migrate apply`) behind a new
  `DialectTooling` snapshot-JSON app seam, and the opt-in `modernc.org/sqlite`
  integration test. These sections are still `[ ]`.

Decisions locked for this build:

- **Scope**: full parity with the PostgreSQL dialect (schema DSL + render +
  snapshot + migration planner incl. table-rebuild + introspection + drift +
  sandbox replay + apply), constrained to what SQLite actually supports.
- **Integration test driver**: `modernc.org/sqlite` (pure-Go, CGo-free) as a
  **test-only** dependency behind an opt-in build tag, mirroring the Docker
  Testcontainers PostgreSQL suite. SQLite needs no container (embedded).

---

## Package Layout (target)

Mirrors `internal/dialects/pg/` and `pg/` exactly (AGENTS.md section 3).

```text
sqlite/                              PUBLIC SQLite DSL (user import path)
  column.go   table.go   schema.go   options.go   defaults.go   sql.go
  registry.go   doc.go
internal/dialects/sqlite/
  sqliteschema/   model types (embed ast.*) + JSON() snapshot function
  render/         SQL renderer + all validation (backtick quoting)
  plan/           structured change IR, diff, table-rebuild planning
  tooling/        PRAGMA introspection, drift, sandbox replay, apply
internal/providers/providers.go      add blank-import of sqlite/
examples/sqlite/                     end-to-end example + goldens
```

Shared, reused as-is (no changes): `internal/ast/`, `kit/`, `internal/app/`,
`internal/cli/`, `internal/migrate/{goose,golangmigrate}`.

---

## Product Boundary

- `[x]` Go-native SQLite schema DSL.
- `[x]` Register SQLite as the second `kit` provider (name `sqlite`, alias
  `sqlite3`).
- `[x]` Deterministic SQLite SQL generation.
- `[x]` Self-describing dialect provider (canonical name, aliases,
  capabilities) — capabilities reflect the SQLite subset (schemas/enums/roles/
  etc. `false`).
- `[x]` No runtime ORM (inherited project boundary).
- `[x]` No query builder (inherited project boundary).
- `[x]` Preserve `sqlc`/`pgx` posture for runtime access — SQLite runtime access
  stays with the user's driver of choice; the DSL pulls in no driver.
- `[x]` Reuse the dialect-neutral CLI generation/snapshot workflow unchanged.
- `[ ]` SQLite migration diff workflow with live and snapshot sources,
  structured plans, risk metadata, destructive guards, rename planning,
  reversible output, and fail-closed unsupported changes.
- `[ ]` Review-first destructive-change handling.
- `[ ]` Snapshot metadata for stable diffs and rename support.

---

## Dialect Provider Boundary

- `[x]` Provider exposes canonical dialect name `sqlite`.
- `[x]` Provider exposes aliases (`sqlite3`).
- `[x]` Provider exposes coarse capabilities reflecting the SQLite subset.
- `[x]` SQL generation routes through the selected provider (`kit.RenderSQL`).
- `[x]` Snapshot generation routes through the selected provider
  (`kit.SnapshotJSON`).
- `[x]` SQLite schema envelope (`sqliteschema.Schema`) separated from the shared
  schema core (`internal/ast`).
- `[x]` SQLite snapshots wrap shared table snapshots with SQLite-only options
  (shadow-embed of `ast.Table`/`ast.Column`/`ast.IndexColumn`).
- `[x]` Shared table/index core already separated from dialect-specific options
  (done for PostgreSQL; SQLite reuses it, proving the boundary).
- `[x]` Dialect capability checks let the CLI report unsupported commands
  cleanly for SQLite (`kit.RequireCapability`).

---

## Schema Object Model

- `[x]` Tables.
- `[x]` Columns.
- `[x]` Views (regular only).
- `[x]` Triggers (`BEFORE`/`AFTER`/`INSTEAD OF`, `FOR EACH ROW`, `WHEN`).
- `[x]` Comments — modelled in the snapshot; table/view/trigger comments rendered
  as `--` SQL comments. **SQLite has no `COMMENT ON`.** (Column comments are
  snapshot-only, not rendered inline.)
- `[x]` Raw SQL schema blocks (escape hatch, additive-only, `.Down()` support) —
  reuse the PostgreSQL pattern for unmodelled DDL (e.g. `PRAGMA`, virtual
  tables).
- `[no]` Schemas/namespaces — SQLite has only `main`/`temp`/attached databases.
  No `CREATE SCHEMA`; all objects live in `main`.
- `[no]` Enums — use `TEXT` + `CHECK (col IN (...))` or a lookup table + FK.
- `[no]` Extensions (PostgreSQL `CREATE EXTENSION`) — SQLite loads extensions at
  the connection level (`load_extension`), not as schema objects.
- `[no]` Sequences — no `CREATE SEQUENCE`; use `INTEGER PRIMARY KEY AUTOINCREMENT`
  (backed by the internal `sqlite_sequence` table).
- `[no]` Domains — no SQLite equivalent; use `CHECK` constraints.
- `[no]` Composite types — no SQLite equivalent.
- `[no]` Materialized views — SQLite has regular views only.
- `[no]` Roles / RLS / policies / grants — SQLite has no user/permission model.
- `[no]` Server-side functions / stored procedures.
- `[no]` Partitioned tables / partitions.
- `[no]` Collations as first-class `CREATE COLLATION` objects — SQLite collations
  (`BINARY`/`NOCASE`/`RTRIM` or app-registered) are referenced inline via
  `COLLATE`, not declared. See Columns.
- `[no]` Tablespaces.

---

## Tables

- `[x]` `CREATE TABLE`.
- `[ ]` `CREATE TABLE IF NOT EXISTS`.
- `[x]` Column list rendering.
- `[x]` Table-level constraint rendering (composite PK, UNIQUE, FK, CHECK).
- `[x]` `WITHOUT ROWID` tables.
- `[x]` `STRICT` tables (strict type affinity enforcement).
- `[x]` Combined `WITHOUT ROWID, STRICT` table options.
- `[x]` Deterministic table ordering respecting FK dependencies (topo-sort with
  alphabetical tie-break).
- `[later]` `TEMP`/`TEMPORARY` tables.

---

## Columns

- `[x]` Column name.
- `[x]` Column type (affinity-mapped declared type).
- `[x]` `NOT NULL`.
- `[x]` Nullable by default.
- `[x]` Column (inline single-column) primary key.
- `[x]` `AUTOINCREMENT` on `INTEGER PRIMARY KEY`.
- `[x]` Inline `UNIQUE`.
- `[x]` `DEFAULT` literal (string/int/float/bool/blob/null).
- `[x]` `DEFAULT` expression (parenthesised, e.g. `CURRENT_TIMESTAMP`).
- `[x]` Safe default helpers (string/int/bool/json-as-text/date/timestamp).
- `[x]` Generated columns `GENERATED ALWAYS AS (expr) STORED`.
- `[x]` Generated columns `GENERATED ALWAYS AS (expr) VIRTUAL`.
- `[x]` Inline `COLLATE` (`BINARY` / `NOCASE` / `RTRIM` / app-registered).
- `[x]` Column comments (snapshot only; see Schema Object Model).
- `[~]` Rename metadata (`previousName`) — carried by `ast.Column`; no DSL setter
  yet (added with rename-aware planning in Slice 5).
- `[x]` Custom-type escape hatch (arbitrary declared type string; SQLite affinity
  rules apply).
- `[no]` Identity columns (`GENERATED ... AS IDENTITY`) — use `AUTOINCREMENT`.
- `[no]` Array columns.

---

## SQLite Types

SQLite uses **type affinity**, not strict storage types. The DSL exposes the
five storage classes plus convenience constructors; since there is no runtime
mapping layer, constructors simply emit the declared type string.

Storage-class constructors:

- `[x]` `Integer` (INTEGER affinity).
- `[x]` `Text` (TEXT affinity).
- `[x]` `Real` (REAL affinity).
- `[x]` `Blob` (BLOB affinity).
- `[x]` `Numeric` (NUMERIC affinity, optional precision/scale).

Convenience constructors (map onto a storage class; no runtime coercion):

- `[x]` `Boolean` → `INTEGER` (0/1).
- `[x]` `Timestamp` → `TEXT` (matches SQLite date functions).
- `[x]` `Int` / `BigInt` → `INTEGER` (aliases for affinity clarity).
- `[x]` `Date` / `Time` → `TEXT`.
- `[x]` `JSON` → `TEXT` (SQLite JSON1 operates on TEXT).
- `[x]` `Any` → `ANY` (STRICT-table wildcard affinity).
- `[x]` `Custom(name, type)` escape hatch (any declared type; STRICT tables
  restrict to the strict type set).

Notes:

- `[~]` `INTEGER PRIMARY KEY` rendered as the rowid alias; planner/introspection
  recognition lands with those phases.
- `[x]` STRICT-table type validation: only `INT`/`INTEGER`/`REAL`/`TEXT`/`BLOB`/
  `ANY` permitted; validated in `validateSchema`.

---

## Constraints

- `[x]` Inline primary key (single column).
- `[x]` Table-level primary key.
- `[x]` Composite primary key (table-level).
- `[x]` Inline unique constraint.
- `[x]` Named table-level unique constraint.
- `[x]` Composite unique constraint.
- `[x]` Inline single-column foreign key.
- `[x]` Named/composite foreign key.
- `[x]` `ON DELETE` action rendering (`CASCADE`/`RESTRICT`/`SET NULL`/`SET
  DEFAULT`/`NO ACTION`).
- `[x]` `ON UPDATE` action rendering.
- `[x]` `DEFERRABLE` / `INITIALLY DEFERRED` foreign keys.
- `[x]` Table check constraints.
- `[x]` Named check helper ergonomics.
- `[~]` FK enforcement guidance: `PRAGMA foreign_keys = ON` is available via the
  raw-SQL escape hatch (see the example); tooling apply/replay enforcement lands
  with those phases.
- `[no]` `NULLS NOT DISTINCT` unique constraints — SQLite treats NULLs as
  distinct; no override.
- `[no]` Exclusion constraints.

---

## Indexes

- `[x]` Basic index (`CREATE INDEX`).
- `[x]` Unique index (`CREATE UNIQUE INDEX`).
- `[x]` Multi-column index by column names.
- `[ ]` `IF NOT EXISTS`.
- `[x]` Named index helpers (`Index`, `UniqueIndex`, `IndexOn`, `IndexExpr`).
- `[x]` Per-column sort direction (`ASC`/`DESC`).
- `[x]` Expression indexes.
- `[x]` Partial indexes (`WHERE`).
- `[x]` Per-column `COLLATE` in index definitions.
- `[no]` Index access methods (`btree`/`gist`/`gin`/…) — SQLite is B-tree only.
- `[no]` Operator classes.
- `[no]` `CONCURRENTLY`.
- `[no]` `ONLY`.
- `[no]` `INCLUDE` covering columns.
- `[no]` Storage parameters (`WITH (...)`).
- `[no]` `NULLS FIRST`/`NULLS LAST` — SQLite fixes NULL ordering (NULLs first in
  ASC); not separately settable.

---

## Views

- `[x]` `CREATE VIEW`.
- `[ ]` `CREATE VIEW IF NOT EXISTS`.
- `[x]` View column list.
- `[x]` View definition SQL / builder.
- `[x]` `TEMP` views (`.Temporary()`).
- `[no]` Materialized views.
- `[no]` View check options (`WITH CHECK OPTION`) — not supported by SQLite.
- `[no]` View security options.

---

## Triggers

- `[x]` `CREATE TRIGGER`.
- `[x]` `BEFORE` / `AFTER` / `INSTEAD OF` timing.
- `[x]` `INSERT` / `UPDATE` / `UPDATE OF (cols)` / `DELETE` events.
- `[x]` `FOR EACH ROW`.
- `[x]` `WHEN` condition.
- `[x]` Trigger body (multi-statement `BEGIN … END`).
- `[x]` `INSTEAD OF` triggers on views (validation enforces `INSTEAD OF` for
  view targets).
- `[~]` Rename metadata (field carried; DSL setter with Slice 5).

---

## Serialisation and Diff Readiness

Reuse the PostgreSQL snapshot design (model IS the snapshot; JSON tags on model
types; deterministic sort before marshal; SHA-256 IDs injected by the app
layer).

- `[x]` `sqliteschema` snapshot format (model serialised, no separate layer).
- `[x]` Snapshot version constant.
- `[x]` Dialect marker (`sqlite`).
- `[x]` Stable snapshot IDs (reuse app-layer SHA-256 injection).
- `[x]` Previous snapshot ID (`snapshot --prev`, app-layer).
- `[x]` Table metadata map.
- `[x]` Column metadata map.
- `[x]` View metadata map.
- `[x]` Trigger metadata map.
- `[x]` Stable object keys.
- `[ ]` Embedded target snapshots in generated migrations.
- `[x]` Deterministic serialisation to JSON.
- `[ ]` Diff input from database introspection.
- `[ ]` Diff input from generated desired snapshot.
- `[ ]` Drift check from database to generated schema.
- `[ ]` Rename annotations (tables/columns/indexes/views/triggers).
- `[ ]` Destructive-change detection.
- `[ ]` Destructive-change default failure mode.
- `[ ]` Explicit override for destructive changes.
- `[no]` Schema/role/function/policy metadata maps — those objects don't exist
  in SQLite.

---

## Migration Engine Slices

Mirror the PostgreSQL slices. Each slice keeps unsupported changes fail-closed
rather than emitting partial SQL. SQLite's headline difference is the very
limited `ALTER TABLE`, which forces a **table-rebuild** for most changes.

### Slice 1: Authoring hardening

- `[x]` Baseline migration generation from the current schema.
- `[x]` Empty migration generation for manual SQL.
- `[x]` SQLite snapshot diffs for additive, destructive, rename, and
  fail-closed replacement changes.
- `[x]` Embedded target snapshot metadata in generated migrations.
- `[x]` Migration directory validation (reuses runner-agnostic validators).
- `[x]` Fail-closed planner errors for unsupported details.
- `[x]` Fixture-style diff tests (additive) and error tests (unsupported).

### Slice 2: Structured planner IR

Reuses the shared `internal/migrate/plan` IR and `SnapshotPlanner` interface;
the SQLite `Planner` plugs into the app's `snapshotPlanner` dispatch.

- `[x]` Changes carry object kind + stable object key.
- `[x]` Create / drop / rename / alter / replace operations.
- `[x]` Dependency metadata.
- `[x]` Reversibility metadata.
- `[x]` Risk flags (`destructive`, `data-loss`, `lock-heavy`,
  `requires-ddl-review`).
- `[x]` Machine-readable plan summary without writing files (`migrate plan
  --json`).
- `[x]` Runner renderer consumes structured plan data.

### Slice 3: Safe additive coverage

- `[x]` `CREATE TABLE` for new tables.
- `[x]` `ADD COLUMN` for existing tables (respecting SQLite's ADD COLUMN limits:
  no non-constant DEFAULT, no `STORED` generated, no PK/UNIQUE, NOT NULL needs a
  DEFAULT — unsafe adds fall back to a rebuild).
- `[x]` `CREATE INDEX` / `CREATE UNIQUE INDEX` for new and existing tables.
- `[x]` `CREATE VIEW` / `CREATE TRIGGER`.
- `[x]` Best-effort down SQL for simple create operations.
- `[x]` Per-change down generation with explicit placeholders for irreversible
  changes (reuses shared runner rendering).

### Slice 4: Destructive-change guardrails

- `[x]` Detect table removals.
- `[x]` Detect column removals (via rebuild; data-loss flagged).
- `[x]` Detect index/view/trigger removals.
- `[x]` Detect column type / nullability / default / generated changes (rebuild).
- `[x]` Detect PK / UNIQUE / FK / CHECK / table-option changes (rebuild).
- `[x]` Risk flags (destructive, data-loss, lock-heavy, requires-ddl-review).
- `[x]` Destructive changes fail by default with actionable diagnostics (app
  layer guard, shared).
- `[x]` Explicit destructive-change override (`--allow-destructive`, shared).

### Slice 5: Rename-aware diffing

- `[x]` Table rename planning (`ALTER TABLE … RENAME TO`).
- `[x]` Column rename planning (`ALTER TABLE … RENAME COLUMN`, SQLite ≥ 3.25).
- `[x]` Index rename planning (drop + create; SQLite cannot rename indexes).
- `[x]` View / trigger rename planning (drop + create).
- `[x]` Rename metadata validation against previous snapshot keys.
- `[x]` Rename-plus-alter combinations fail closed until semantic planning
  supports them.

### Slice 5b: Table-rebuild (the SQLite headline)

The generalised `ALTER TABLE` procedure for changes SQLite cannot do in place
(column type/constraint changes, drops, PK/FK/CHECK/UNIQUE constraint changes,
STRICT/WITHOUT ROWID option changes, adding STORED generated columns).

- `[x]` Detect changes that require a rebuild vs. a direct `ALTER`.
- `[x]` Emit `PRAGMA foreign_keys=OFF;` … `PRAGMA foreign_keys=ON;` wrapping.
- `[x]` `CREATE TABLE <new>` with the target schema.
- `[x]` `INSERT INTO <new> (cols) SELECT cols FROM <old>` copy (follows column
  rename metadata; skips generated and brand-new columns).
- `[x]` `DROP TABLE <old>`.
- `[x]` `ALTER TABLE <new> RENAME TO <old>`.
- `[x]` Recreate indexes and triggers on the rebuilt table (the rebuild owns
  them in both directions; the trigger pass skips rebuilt tables). Views survive
  the same-name swap.
- `[x]` `lock-heavy` + `requires-ddl-review` risk metadata, plus
  `destructive`/`data-loss` when columns are dropped.
- `[x]` Best-effort reverse rebuild to the previous table definition.
- `[later]` `PRAGMA foreign_key_check` verification posture (deferred to tooling
  apply/replay).

### Slice 6: Sandbox replay and drift check

- `[ ]` SQLite connection plumbing for tooling commands (file / in-memory DSN).
- `[ ]` PRAGMA-based introspection → snapshot-compatible model
  (`sqlite_schema`, `pragma_table_info`, `pragma_table_xinfo`,
  `pragma_index_list`, `pragma_index_info`, `pragma_foreign_key_list`, parse of
  `sqlite_schema.sql` for checks/generated/collation).
- `[ ]` Sandbox replay of committed migrations (disposable file / `:memory:`).
- `[ ]` Replay comparison against the latest embedded target snapshot.
- `[ ]` `migrate check --sandbox-url` for SQLite.
- `[ ]` `drift check --url` for SQLite.
- `[ ]` Deterministic introspection output with drift projection normalising
  affinity aliases, rowid PK, default opclass/collation, and SQLite expression
  rewrites.

### Slice 7: Apply and runner

- `[ ]` `migrate apply --url` for SQLite (goose version table + skip semantics).
- `[ ]` `golang-migrate` renderer works for SQLite (reuse dialect-neutral
  renderer; verify `CREATE INDEX` split behaviour and PRAGMA statements).
- `[ ]` Machine-readable JSON output for SQLite commands.
- `[ ]` Quiet output mode for SQLite commands.

---

## Database Connectivity and Auth

SQLite is file-embedded; connectivity is a file path (or `:memory:`), not a
network session. The provider-neutral auth interface exists but most token
providers are not applicable.

- `[ ]` Connect to SQLite via a file path / DSN (`file:...`, `:memory:`).
- `[ ]` Respect SQLite DSN options (e.g. `_pragma=foreign_keys(1)`, busy
  timeout) needed by tooling.
- `[ ]` Keep the DSL/renderer free of any SQLite driver (driver is test-only /
  tooling-only).
- `[ ]` Enforce `PRAGMA foreign_keys` posture during apply/replay.
- `[no]` Password / SSL-TLS options — not applicable to a local file.
- `[no]` Token-as-password auth, Azure Entra, AWS IAM, GCP IAM, OAuth2 token
  providers — no network auth surface. The shared `--auth` selection reports
  these as unsupported for SQLite.
- `[later]` Encrypted SQLite (SQLCipher / SEE) connection options.

---

## CLI and Workflow

These commands already exist and are dialect-neutral; SQLite work is making them
resolve the SQLite provider and dispatch correctly, plus capability gating for
the unsupported ones.

- `[ ]` `gosqlkit generate` (and `--out`, `--check`) with `dialect: sqlite`.
- `[ ]` `gosqlkit snapshot` (and `--out`, `--check`, `--prev`).
- `[ ]` `gosqlkit inspect --url <sqlite>` introspection to snapshot JSON.
- `[ ]` `gosqlkit migrate create <name>` (additive/destructive/rename, risk
  flags, best-effort down).
- `[ ]` `gosqlkit migrate create <name> --empty`.
- `[ ]` `gosqlkit migrate create <name> --allow-destructive`.
- `[ ]` `gosqlkit migrate plan` (`--json`, `--quiet`).
- `[ ]` `gosqlkit migrate check` (`--sandbox-url`).
- `[ ]` `gosqlkit migrate apply --url <sqlite>`.
- `[ ]` `gosqlkit drift check --url <sqlite>`.
- `[ ]` Object-level drift diagnostics (missing/extra/changed) in human + JSON.
- `[ ]` Styled terminal tables for SQLite drift and plan summaries (reuse
  lipgloss helpers).
- `[ ]` Interactive rename-candidate prompts for SQLite objects (tables,
  columns, indexes, views, triggers).
- `[ ]` Non-interactive fail-closed ambiguity diagnostics.
- `[ ]` Goose-compatible migration output for SQLite.
- `[ ]` `golang-migrate`-compatible migration output for SQLite.
- `[ ]` `gosqlkit ci schema` / `gosqlkit ci database` for SQLite.
- `[ ]` Shell completion already covers commands (dialect-neutral; no work).
- `[no]` `migrate refresh <matview>` — SQLite has no materialized views.
- `[no]` `--token-command`, `--auth azure-entra|aws-iam|gcp-iam` for SQLite.

---

## Validation

- `[x]` Invalid identifier validation (`^[a-z_][a-z0-9_]*$`, matching the
  PostgreSQL dialect's convention).
- `[x]` Duplicate table / column / index / check / constraint validation.
- `[x]` Foreign key action validation.
- `[x]` Unknown local/index constraint column validation.
- `[x]` Unknown referenced table / column validation.
- `[x]` STRICT-table type validation (allowed type set).
- `[x]` `AUTOINCREMENT` only on `INTEGER PRIMARY KEY` validation.
- `[x]` Generated-column: `STORED` vs `VIRTUAL`, no `DEFAULT` and no `PRIMARY
  KEY` alongside.
- `[~]` Invalid default / generated expression validation where practical
  (non-empty generated expression enforced; deeper checks later).
- `[x]` `WITHOUT ROWID` requires a PRIMARY KEY validation.
- `[x]` Actionable validation diagnostics.

---

## Testing Requirements

- `[x]` Renderer golden tests (self-contained, decoupled from the example).
- `[x]` CLI generation test for SQLite (via the registry render/snapshot test
  and the checked-in example goldens under `verify:generate`/`verify:cli`).
- `[x]` Snapshot golden tests (`sqliteschema` shape test + example snapshot).
- `[x]` Constraint rendering tests (in the golden).
- `[x]` Index rendering tests (partial, expression, desc, collate — in the
  golden).
- `[x]` Default literal rendering tests (in the golden / registry test).
- `[x]` Generated-column rendering tests (STORED/VIRTUAL — in the golden).
- `[x]` View rendering tests (in the golden).
- `[x]` Trigger rendering tests (in the golden).
- `[x]` `WITHOUT ROWID` / `STRICT` rendering tests (in the golden + rejection
  tests).
- `[ ]` Diff fixture tests (additive).
- `[ ]` Destructive-change fixture tests.
- `[ ]` Rename-aware diff tests.
- `[ ]` Table-rebuild planning tests (the 12-step procedure).
- `[ ]` Introspection → snapshot tests.
- `[ ]` Drift normalisation matrix tests (affinity aliases, rowid PK, default
  collation, expression rewrites).
- `[ ]` **Opt-in `modernc.org/sqlite` integration test**: apply generated
  schema, introspect, drift-check, and sandbox-replay against a real embedded
  SQLite database (satisfies FEATURES.md "Testcontainers integration test per
  supported dialect" — SQLite is embedded, no container needed).
- `[ ]` Integration test for `migrate apply` version tracking on SQLite.
- `[ ]` Down-SQL replay tests for reversible generated migrations.
- `[ ]` Table-rebuild replay test (round-trips data through a rebuild).

---

## Recommended Build Order

1. `[x]` `sqliteschema` model + `JSON()` snapshot (reuse `ast`, add SQLite-only
   fields via shadow-embed).
2. `[x]` `render` package + `validateSchema` + golden tests.
3. `[x]` Public `sqlite/` DSL (columns, tables, constraints, indexes, views,
   triggers) + provider registration + `internal/providers` wiring.
4. `[x]` `examples/sqlite/` + generated SQL/snapshot goldens + Taskfile tasks.
   **← proves the dialect boundary; flips FEATURES.md line 68. DONE.**
5. `[x]` Migration planner (`plan/`): structured IR → additive → destructive
   guards → rename → table-rebuild. **DONE** (`migrate create`/`migrate plan`
   work for SQLite; generic diff helpers lifted into `internal/migrate/plan` and
   PG refactored onto them).
6. `[ ]` Tooling (`tooling/`): PRAGMA introspection → drift projection → sandbox
   replay → `migrate apply`.
7. `[ ]` Opt-in `modernc.org/sqlite` integration test + docs (FEATURES.md,
   AGENTS.md, README) + full `task verify`.
   **← flips FEATURES.md line 612.**

---

## Cross-References to FEATURES.md

When SQLite lands, flip these PostgreSQL-board items:

- FEATURES.md "Dialect Provider Boundary" → `[ ]` First non-PostgreSQL provider
  proving the boundary → `[x]` (after build order step 4).
- FEATURES.md "Testing Requirements" → `[ ]` Testcontainers integration test per
  supported dialect → `[x]` (after build order step 7; note SQLite is embedded).

Update this SQLITE.md board as items land; keep AGENTS.md "Current state at a
glance" in sync when SQLite reaches usable milestones.
