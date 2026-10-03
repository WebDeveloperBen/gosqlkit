# Migration Generation Design

This document records the migration-generation decisions for `gosqlkit`: a
cross-dialect plan model, dialect-specific planning, and runner-compatible
artifacts for PostgreSQL and SQLite.

## Decision

`gosqlkit` will own a cross-dialect migration planning layer instead of
delegating the core migration diff to a PostgreSQL-only tool.

The generated migration artefact should be runner-compatible SQL, not a
`gosqlkit`-specific migration runtime format. Migration planning and migration
file rendering are separate layers:

```text
snapshot diff -> migration plan / change IR -> runner-specific file renderer
```

The initial and default file renderer is `goose`. It emits one committed SQL
file per migration with machine-readable `gosqlkit` metadata embedded in SQL
comments. `golang-migrate` is also supported as a split-file renderer, with
metadata embedded in the `.up.sql` file. We will not create per-migration
sidecar JSON files or a directory checksum file by default.

The migration configuration lets a project choose `goose` or
`golang-migrate`. Unsupported runner values should fail validation rather than
silently producing the wrong file layout.

Migration application stays compatible with supported runner layouts.
`gosqlkit migrate apply` is an optional wrapper around the same files, not a
prerequisite for using goose or another runner.

## Why This Path

The project needs to support more than PostgreSQL. PostgreSQL-only tools such
as `pg-schema-diff` and `pgschema` are useful references and possible test
oracles, but they cannot be the centre of the architecture if MySQL, SQLite,
MSSQL, SingleStore, CockroachDB, and other dialects are first-class goals.

Drizzle Kit is the strongest reference for the schema-as-code workflow:

- Generate a structured snapshot from schema files.
- Diff the current snapshot against a previous snapshot.
- Generate SQL migrations from the semantic difference.
- Prompt for rename intent where automatic detection is unsafe.

Atlas is the strongest reference for migration discipline:

- Separate declarative desired state from versioned migration files.
- Validate generated plans against a real database engine where possible.
- Treat database-specific DDL planning as dialect-specific.
- Provide migration directory checks and apply workflows.

`pgschema` is a strong PostgreSQL planning reference:

- It has deep PostgreSQL object coverage.
- It treats migrations as desired-state planning.
- It shows how much dialect-specific behaviour exists below the common schema
  abstraction.

The `gosqlkit` decision is to borrow these concepts, but keep our own core
snapshot, diff, and migration IR so every dialect can participate.

## Reference Implementation Research

Atlas and Drizzle Kit are architecture references, not dependencies. Do not
shell out to Atlas for planning and do not import Atlas or Drizzle code into
`gosqlkit`. The reason for building `gosqlkit` is to keep the Go schema
workflow free, reviewable, and owned by this project.

Use the public source of both projects to understand proven migration-planning
patterns before expanding this feature:

- Atlas pattern: typed `schema.Change` IR plus dialect-specific diff/planning.
  Reference:
  `https://github.com/ariga/atlas/blob/master/sql/schema/migrate.go`
- Atlas PostgreSQL planner pattern: the dialect planner turns typed changes
  into ordered SQL, handles reversibility, and separates table-level changes
  from independent index/comment/object statements. Reference:
  `https://github.com/ariga/atlas/blob/master/sql/postgres/migrate.go`
- Drizzle pattern: the snapshot differ produces structured JSON statements,
  then the SQL generator renders those statements. Reference tree:
  `https://github.com/drizzle-team/drizzle-orm/tree/main/drizzle-kit/src`

When working on this feature, clone references into `/tmp` and inspect them
locally so the repo stays clean:

```bash
rtk git clone --depth 1 https://github.com/ariga/atlas.git /tmp/gosqlkit-atlas-ref
rtk git clone --depth 1 https://github.com/drizzle-team/drizzle-orm.git /tmp/gosqlkit-drizzle-ref
```

Useful files and search starting points:

```bash
rtk sed -n '1,260p' /tmp/gosqlkit-atlas-ref/sql/schema/migrate.go
rtk sed -n '1,360p' /tmp/gosqlkit-atlas-ref/sql/postgres/diff.go
rtk sed -n '1,420p' /tmp/gosqlkit-atlas-ref/sql/postgres/migrate.go
rtk sed -n '1490,1665p' /tmp/gosqlkit-atlas-ref/sql/postgres/inspect.go
rtk sed -n '172,245p' /tmp/gosqlkit-atlas-ref/sql/sqltool/tool.go
rtk rg -n "applyJsonDiff|JsonStatement|alter_table|rename_table|drop_table" /tmp/gosqlkit-drizzle-ref/drizzle-kit/src
rtk sed -n '1388,1410p' /tmp/gosqlkit-drizzle-ref/drizzle-kit/src/serializer/pgSerializer.ts
rtk sed -n '1538,1572p' /tmp/gosqlkit-drizzle-ref/drizzle-kit/src/serializer/pgSerializer.ts
rtk sed -n '1,260p' /tmp/gosqlkit-drizzle-ref/drizzle-kit/src/jsonStatements.ts
rtk sed -n '1,260p' /tmp/gosqlkit-drizzle-ref/drizzle-kit/src/jsonDiffer.js
```

Translate the ideas into `gosqlkit`'s own model:

- `internal/migrate/plan` owns the shared Go interfaces and change metadata.
- `internal/dialects/<dialect>/plan` owns snapshot comparison, dialect
  ordering, reversibility rules, risk flags, and SQL rendering.
- `internal/migrate/goose` and `internal/migrate/golangmigrate` own runner
  file layout only.
- `internal/app/migrate.go` orchestrates config, previous metadata lookup,
  planner selection, file rendering, and write/check behaviour.

For live PostgreSQL validation, follow the same shape Atlas and Drizzle use:
read semantic catalogue fields instead of diffing rendered SQL text. In
practice that means identity/generated-column flags, `pg_get_expr` predicates,
`pg_get_indexdef` expressions, index option bits for direction and null
ordering, opclasses, storage parameters, and extension-owned object filters.
For migration replay, split goose `Up` SQL after section extraction and split
`golang-migrate` `.up.sql` files directly so non-transactional statements such
as `CREATE INDEX CONCURRENTLY` are not forced into a single batch, while
function bodies and quoted semicolons remain intact. Goose files whose plan
contains a non-transactional risk carry Goose's native `NO TRANSACTION`
annotation so external Goose does not wrap those statements.

## Non-Goals

- Do not build a runtime ORM.
- Do not build a query builder.
- Do not make a PostgreSQL-only diff engine the cross-dialect core.
- Do not require a permanent shadow database to author migrations.
- Do not commit environment-specific "applied migrations" state to Git.
- Do not generate a pile of sidecar files for every migration by default.
- Do not promise safe automatic down migrations for every change.

## Desired User Experience

Generate a migration from the Go schema:

```bash
gosqlkit migrate create add_rls_policies
```

Apply it using goose:

```bash
goose -dir db/migrations postgres "$DATABASE_URL" up
```

Or apply the same runner-compatible files through `gosqlkit`:

```bash
gosqlkit migrate apply --url "$DATABASE_URL"
```

`migrate apply` records successful versions in the runner's standard table
(`goose_db_version` for goose, `schema_migrations` for `golang-migrate`) and
skips versions that are already applied. The generated SQL remains plain,
reviewable, and usable without a custom runner.
For `golang-migrate`, `schema_migrations` is treated as current state rather
than append-only history: apply fails when the current row is dirty, marks the
target version dirty before execution, and records the target version clean
after successful execution.

For PostgreSQL, transaction-compatible migration statements and the successful
runner-state update commit in one transaction. A failed transactional migration
rolls back its DDL and state change. `golang-migrate`'s dirty marker is committed
before execution and remains dirty on failure.

PostgreSQL statements that cannot run in a transaction execute directly. Goose
records the version as unapplied before execution and marks it applied only
after success; `golang-migrate` retains its dirty version if execution fails.
These markers require manual inspection and repair before retrying: prior
non-transactional statements may have taken effect, so failure is not an atomic
rollback. Opaque `raw_sql` that cannot be classified for transaction safety is
rejected before migration execution.

## Migration File Layout

Default `goose` layout:

```text
db/migrations/
  20260706143000_add_rls_policies.sql
```

No default sidecars:

```text
20260706143000_add_rls_policies.plan.json       # not generated by default
20260706143000_add_rls_policies.snapshot.json   # not generated by default
gosqlkit.sum                                    # not generated by default
```

The SQL file carries both executable SQL and enough metadata to reconstruct
lineage:

```sql
-- +gosqlkit Meta
-- {
--   "version": 1,
--   "dialect": "postgresql",
--   "fromSnapshotId": "abc...",
--   "toSnapshotId": "def...",
--   "targetSnapshot": {
--     "snapshotId": "def..."
--   },
--   "createdAt": "2026-07-06T14:30:00Z",
--   "changes": [
--     {"op":"alter","object":{"kind":"table","key":"public.users"}},
--     {"op":"create","object":{"kind":"policy","key":"public.users.users_read_self"}}
--   ]
-- }

-- +goose Up
ALTER TABLE users ENABLE ROW LEVEL SECURITY;

CREATE POLICY users_read_self ON users
FOR SELECT
TO app_reader
USING (id = current_setting('app.user_id')::uuid);

-- +goose Down
DROP POLICY users_read_self ON users;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
```

This keeps Git review focused on one file per migration while preserving
machine-readable context. Generated migrations embed the target snapshot in
metadata so the latest committed migration can serve as the previous desired
schema for the next diff. Empty manual migrations may omit the snapshot payload
until manual-result metadata is supported.

The `golang-migrate` renderer uses a split-file layout while preserving the
same `gosqlkit` plan and metadata concepts:

```text
db/migrations/
  20260706143000_add_rls_policies.up.sql
  20260706143000_add_rls_policies.down.sql
```

For PostgreSQL compatibility with the external `golang-migrate` CLI, generated
`golang-migrate` up files are split into numbered single-statement versions
when a rendered plan contains multiple PostgreSQL statements:

```text
db/migrations/
  202607061430000001_baseline.up.sql
  202607061430000002_baseline.up.sql
```

This keeps PostgreSQL function bodies with semicolons intact and preserves
non-transactional statements such as `CREATE INDEX CONCURRENTLY`, because each
statement is sent to PostgreSQL as its own migration file instead of being
forced into an implicit multi-statement transaction.

Runner-specific annotations, filename rules, and one-file versus two-file
output belong in the file renderer. The migration plan, risk metadata,
destructive-change handling, dependency ordering, and dialect SQL planning
must stay independent of the runner format.

## Applied State

Applied migration state is environment-specific and belongs in the database,
not in Git.

`gosqlkit migrate apply` records successful versions in the selected runner's
standard table (`goose_db_version` or `schema_migrations`). It does not create a
second project-specific migration history table.

Git should contain migrations that are available to apply. It should not
contain a JSON list of migrations that have been applied, because development,
staging, and production are often intentionally at different versions.

## Migration Source Decisions

Migration creation can use the latest embedded migration snapshot or an
explicit live database source; snapshot-based input remains the default.

1. **Previous migration metadata**

   Default for normal development. Read the latest committed migration's
   embedded `toSnapshotId` and metadata, compare the previous desired snapshot
   to the new desired snapshot, and generate the next migration.

2. **Sandbox replay**

   Apply migrations into a temporary dialect-matched database, introspect the
   result, and compare it to the latest desired snapshot. This validates that
   committed migrations still produce the expected schema.

3. **Live database introspection**

Live introspection supports PostgreSQL drift checks and explicit migration
sources. It should not be the default authoring path because it depends on
access to a specific environment. SQLite live-source planning is tracked in the
active [`sqlite-tooling` change](openspec/changes/sqlite-tooling/tasks.md).

## Sandbox Databases

Real database engines still matter in 2026. SQL parsing alone is not enough to
validate defaults, generated expressions, opclasses, extensions, procedural
objects, lock behaviour, transactional DDL, and dialect-specific `ALTER`
support.

`gosqlkit` should use temporary sandbox databases for validation and, where
needed, dialect planning. The sandbox is not the source of truth. It is a
validation tool.

For PostgreSQL, this could be a local Docker database, a testcontainer, an
embedded temporary PostgreSQL process, or an explicitly configured dev URL. For
SQLite, it may be an in-memory database. Other dialects can choose the
smallest realistic engine setup that validates their DDL.

PostgreSQL sandbox replay behavior and validation are specified in the
[PostgreSQL workflow contract](openspec/specs/postgres-schema-workflow/spec.md).
SQLite sandbox replay remains part of the active
[`sqlite-tooling` change](openspec/changes/sqlite-tooling/tasks.md).

## Internal Architecture

Proposed package shape:

```text
internal/migrate/
  Migration metadata parser/writer
  Migration directory scanning
  Metadata chain validation
  Runner-neutral migration file model
  Runner renderer registry

internal/migrate/plan/
  Shared planner interfaces
  Change IR
  Object keys
  Risk flags
  Dependency ordering primitives

internal/migrate/goose/
  Goose-compatible file rendering

internal/migrate/golangmigrate/
  golang-migrate split-file rendering

internal/dialects/<dialect>/plan/
  Snapshot-to-snapshot diff rules
  Dialect operation ordering
  Reversibility rules
  SQL statement rendering

internal/dialects/<dialect>/inspect/
  Database -> snapshot

internal/dialects/<dialect>/validate/
  Apply SQL to sandbox
  Introspect and compare

internal/app/migrate.go
  create/check/apply orchestration
```

The renderer boundary should be intentionally small. A renderer receives a
planned migration and returns one or more files to write. `goose` returns one
file containing `-- +goose Up` and optional `-- +goose Down` sections.
`golang-migrate` returns separate `.up.sql` and `.down.sql` files.

The planner boundary follows the same split used by Atlas and Drizzle Kit:
compare structured snapshots into typed changes first, then let the dialect
planner order and render those changes into SQL. `internal/migrate/plan`
contains the shared interfaces and change metadata; each dialect owns its
snapshot comparison and SQL planning details under
`internal/dialects/<dialect>/plan/`.

The core change IR should be dialect-neutral enough for command UX and review:

```text
Create
Drop
Rename
Alter
Replace
```

Each change should carry:

- Stable object key.
- Object kind.
- Dialect.
- Operation.
- Dependencies.
- Reversibility.
- Risk flags.
- Human-readable summary.

Risk flags should include:

- destructive
- data-loss
- lock-heavy
- non-transactional
- requires-backfill
- manual-review

Dialect planners decide how the IR maps to concrete DDL.

## Down Migrations

Down migrations are best-effort and explicit.

Generate `Down` SQL when a change is clearly reversible:

- Create index -> drop index.
- Create policy -> drop policy.
- Create view -> drop view.
- Rename with explicit previous name -> rename back.

Generate warnings or omit automatic down SQL when reversal is ambiguous or
unsafe:

- Type changes.
- Data transformations.
- Destructive column/table drops.
- Procedural changes with side effects.
- Dialect operations that cannot be reversed transactionally.

For production, forward-fix migrations are the expected recovery model.
Generated downs are mainly for local development, tests, and clearly safe
schema object reversals.

## Migration Directory Validation

We are not adopting Atlas's `atlas.sum` file by default. It is a useful
integrity mechanism, but it intentionally creates merge conflicts and adds a
second committed file to maintain.

Migration metadata, runner annotations, snapshot lineage, and sandbox replay
are part of the migration workflow. Their current behavior is specified in the
published PostgreSQL capability contract; implementation work for other
dialects belongs in the corresponding active OpenSpec change.

## Commands

Example command shape:

```bash
gosqlkit migrate create <name>
gosqlkit migrate create <name> --empty
gosqlkit migrate create <name> --no-down
gosqlkit migrate check
gosqlkit migrate check --sandbox-url ...
gosqlkit drift check --url "$DATABASE_URL"
gosqlkit migrate apply --url "$DATABASE_URL"
gosqlkit migrate apply --url-env DATABASE_URL
gosqlkit migrate apply --token-command "az account get-access-token --resource-type oss-rdbms --query accessToken -o tsv"
gosqlkit migrate apply --quiet
```

`migrate apply` is a convenience wrapper around the same migration files. It
should not block users from applying with goose or another runner.

Configuration should reserve space for runner selection:

```yaml
migrations:
  dir: db/migrations
  runner: goose
```

Omitting `runner` is equivalent to `goose`. Supported values are `goose` and
`golang-migrate`.

## Current Contracts and Work

Published OpenSpec capability specs define supported migration behavior.
Active OpenSpec changes contain unfinished migration work and ordered tasks.
Use `openspec list --specs --json`, `openspec list --json`, and
`openspec status --change <name> --json` for current status. The
[PostgreSQL workflow](openspec/specs/postgres-schema-workflow/spec.md),
[SQLite workflow](openspec/specs/sqlite-schema-workflow/spec.md), and active
[`sqlite-tooling` change](openspec/changes/sqlite-tooling/tasks.md) provide the
current behavior and implementation references. This document remains a
decision record, not a release-phase backlog.

## Reasons For This Decision

- Multi-dialect support requires our own core migration model.
- PostgreSQL-specific tools remain valuable as references, validators, and
  possible implementation shortcuts, but not as the abstraction boundary.
- One SQL file per migration keeps Git review clean.
- Embedded metadata avoids sidecar churn while preserving machine context.
- Embedding the generated target snapshot lets future diff creation use the
  latest migration as the previous desired schema without requiring a sidecar
  snapshot file.
- Keeping runner rendering separate from migration planning lets the project
  add `golang-migrate` or another file layout later without changing the diff
  engine.
- Database-applied state belongs in the database, not in committed JSON.
- Goose compatibility gives a mature runner immediately.
- Sandbox validation keeps generated SQL honest without making the sandbox the
  source of truth.
- Best-effort down migrations avoid pretending that all schema changes are
  safely reversible.

## Open Questions

- Should `gosqlkit migrate create` require a sandbox for all dialects, or only
  for operations that cannot be validated from the model?
- How should manual migrations declare their resulting snapshot?
- Should data migrations be represented as manual SQL blocks with explicit
  risk metadata?
- Should projects be able to opt into Atlas-style hash files?
