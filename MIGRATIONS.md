# Migration Generation Design

This document records the current migration-generation direction for
`gosqlkit`. It is the decision note for how Go schema files become reviewable
database migrations across PostgreSQL first, and other dialects later.

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
comments. We will not create per-migration sidecar JSON files or a directory
checksum file by default.

The migration configuration should be shaped so a project can eventually
choose a runner format, for example `goose` or `golang-migrate`. Only `goose`
is implemented at first; unsupported runner values should fail validation
rather than silently producing the wrong file layout.

Initial migration application should stay compatible with `goose`. A future
`gosqlkit migrate apply` command may wrap application, but it should not be a
prerequisite for using generated migrations.

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
rtk rg -n "applyJsonDiff|JsonStatement|alter_table|rename_table|drop_table" /tmp/gosqlkit-drizzle-ref/drizzle-kit/src
rtk sed -n '1,260p' /tmp/gosqlkit-drizzle-ref/drizzle-kit/src/jsonStatements.ts
rtk sed -n '1,260p' /tmp/gosqlkit-drizzle-ref/drizzle-kit/src/jsonDiffer.js
```

Translate the ideas into `gosqlkit`'s own model:

- `internal/migrate/plan` owns the shared Go interfaces and change metadata.
- `internal/dialects/<dialect>/plan` owns snapshot comparison, dialect
  ordering, reversibility rules, risk flags, and SQL rendering.
- `internal/migrate/goose` and future `internal/migrate/golangmigrate` own
  runner file layout only.
- `internal/app/migrate.go` orchestrates config, previous metadata lookup,
  planner selection, file rendering, and write/check behaviour.

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

Later, `gosqlkit` can provide a wrapper:

```bash
gosqlkit migrate apply --url "$DATABASE_URL"
```

but the generated SQL should remain plain, reviewable, and usable without a
custom runner.

Longer term, the same migration plan should be renderable for other runners
without changing the diff planner. For example, a future `golang-migrate`
renderer can emit split `.up.sql` and `.down.sql` files from the same plan that
the `goose` renderer emits as a single annotated SQL file.

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

A future `golang-migrate` renderer would use a different file layout while
preserving the same `gosqlkit` plan and metadata concepts:

```text
db/migrations/
  20260706143000_add_rls_policies.up.sql
  20260706143000_add_rls_policies.down.sql
```

Runner-specific annotations, filename rules, and one-file versus two-file
output belong in the file renderer. The migration plan, risk metadata,
destructive-change handling, dependency ordering, and dialect SQL planning
must stay independent of the runner format.

## Applied State

Applied migration state is environment-specific and belongs in the database,
not in Git.

When using goose, goose's migration table is the source of applied state. If
`gosqlkit` later owns application directly, it should create its own migration
history table, such as `gosqlkit_schema_migrations`.

Git should contain migrations that are available to apply. It should not
contain a JSON list of migrations that have been applied, because development,
staging, and production are often intentionally at different versions.

## Current State Inputs

Migration creation should support three source modes.

1. **Previous migration metadata**

   Default for normal development. Read the latest committed migration's
   embedded `toSnapshotId` and metadata, compare the previous desired snapshot
   to the new desired snapshot, and generate the next migration.

2. **Sandbox replay**

   Apply migrations into a temporary dialect-matched database, introspect the
   result, and compare it to the latest desired snapshot. This validates that
   committed migrations still produce the expected schema.

3. **Live database introspection**

   Used for drift checks and reconciliation. Compare a live database's
   introspected snapshot to the desired snapshot. This should not be the
   default authoring path because it depends on access to a specific
   environment.

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
  Future golang-migrate split-file rendering

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
`golang-migrate` can later return separate `.up.sql` and `.down.sql` files.

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

Instead, `gosqlkit migrate check` should validate:

- `[x]` Migration filenames are timestamped.
- `[x]` Embedded metadata is parseable.
- `[x]` Embedded target snapshot IDs match `toSnapshotId` when target snapshots
  are present.
- `[x]` Goose files contain exactly one `-- +goose Up` annotation, at most one
  `-- +goose Down` annotation, and `Down` appears after `Up`.
- `[x]` Adjacent snapshot lineage is coherent when both sides declare
  snapshot IDs.
- No migration appears before the latest applied migration for a target
  database.
- Replaying migrations in a sandbox reaches the expected final snapshot.
- Manual edits do not break statement parsing or declared snapshot lineage.

If later experience shows that projects need stronger directory integrity, we
can add an optional hash file. It should not be the default UX.

## Commands

Planned command shape:

```bash
gosqlkit migrate create <name>
gosqlkit migrate create <name> --empty
gosqlkit migrate create <name> --no-down
gosqlkit migrate check
gosqlkit migrate check --sandbox-url ...
gosqlkit drift check --url "$DATABASE_URL"
gosqlkit migrate apply --url "$DATABASE_URL"
```

`migrate apply` should be a later wrapper around the same migration files. It
should not block users from applying with goose or another runner.

Configuration should reserve space for runner selection:

```yaml
migrations:
  dir: db/migrations
  runner: goose
```

For the first implementation, omitting `runner` is equivalent to `goose`, and
any value other than `goose` is invalid.

Current implementation status:

- `migrate create <name>` creates a baseline migration from the current schema
  when the migration directory has no existing migrations. The generated
  metadata embeds both `toSnapshotId` and the target snapshot JSON.
- `migrate create <name> --empty` creates an empty goose-compatible migration
  for manual SQL.
- `migrate create <name>` with existing migrations uses the latest migration's
  embedded target snapshot as the previous state and currently supports
  conservative additive PostgreSQL changes: new schemas, extensions, enums,
  appended enum values, new tables, new columns, standalone indexes, and
  table/column comments, plus primary-key, unique, foreign-key, and check
  constraints on existing tables, and exclusion constraints on new and
  existing tables. It also supports enabling/forcing table RLS and creating
  RLS policies, plus creating roles, sequences, composite types, domains, and
  functions.
- Destructive changes and unsupported modifications fail closed with an
  explicit planner error.
- `migrate check` validates timestamped SQL filenames, embedded metadata, and
  goose annotations, plus adjacent snapshot lineage when both migrations
  declare snapshot IDs.
- Fixture tests cover supported additive v0 diff operations and the current
  unsupported/destructive planner error categories.
- The internal planner IR now carries typed object references, typed
  dependencies, reversibility, risk flags, and per-change forward/reverse SQL
  statements while preserving the v0 statement slice used by compatibility
  paths.
- The goose renderer consumes runner-neutral structured SQL statements from
  migration plans, with raw SQL slices retained only as a compatibility
  fallback.
- Safe additive PostgreSQL diff coverage includes standalone index creation on
  new and existing tables, with table dependencies and reverse `DROP INDEX`
  statements recorded in the structured plan.
- Safe additive PostgreSQL diff coverage includes table and column comment
  creation, with reverse `COMMENT ... IS NULL` statements recorded in the
  structured plan.
- Safe additive PostgreSQL diff coverage includes table-level primary-key,
  unique, foreign-key, and check constraint creation on existing tables, with
  table dependencies and reverse `ALTER TABLE ... DROP CONSTRAINT` statements
  recorded in the structured plan.
- Safe additive PostgreSQL diff coverage includes exclusion constraints on new
  and existing tables, including method, element, partial predicate, and
  deferrability options. Existing-table additions record reverse
  `ALTER TABLE ... DROP CONSTRAINT` statements.
- Safe additive PostgreSQL diff coverage includes enabling and forcing table
  RLS, with reverse `DISABLE ROW LEVEL SECURITY` and
  `NO FORCE ROW LEVEL SECURITY` statements. Disabling either state remains a
  destructive change and fails closed.
- Safe additive PostgreSQL diff coverage includes RLS policy creation, with
  table dependencies and reverse `DROP POLICY ... ON ...` statements.
- Safe additive PostgreSQL diff coverage includes role creation, with reverse
  `DROP ROLE` statements.
- Safe additive PostgreSQL diff coverage includes sequence creation, with
  reverse `DROP SEQUENCE` statements. `OWNED BY` is emitted as a separate
  post-table `ALTER SEQUENCE ... OWNED BY ...` change so sequence defaults can
  be available before table creation while ownership waits for the referenced
  column to exist.
- Safe additive PostgreSQL diff coverage includes composite type creation,
  with reverse `DROP TYPE` statements.
- Safe additive PostgreSQL diff coverage includes domain creation, with reverse
  `DROP DOMAIN` statements.
- Safe additive PostgreSQL diff coverage includes function creation and
  function comments, keyed by identity arguments for overloaded functions, with
  reverse `DROP FUNCTION` and `COMMENT ... IS NULL` statements.
- Safe additive PostgreSQL diff coverage includes trigger creation and trigger
  comments, with dependencies on the trigger target and trigger function and
  reverse `DROP TRIGGER` / `COMMENT ... IS NULL` statements.
- Safe additive PostgreSQL diff coverage includes view and materialized view
  creation and comments, with dependency refs from `DependsOn` and reverse
  `DROP VIEW` / `DROP MATERIALIZED VIEW` statements.

## Next Migration Engine Slices

Ship the migration engine as reviewable vertical slices. Each slice should
leave the CLI usable, fail closed for unsupported changes, and add fixture
tests before broadening the planner.

### Slice 1: Authoring V0 Hardening

Goal: make the staged `migrate create` / `migrate check` workflow safe enough
to release as the first migration-authoring loop.

Scope:

- Keep baseline migration generation from the current schema.
- Keep empty goose migration generation for manual SQL.
- Keep conservative additive PostgreSQL diffs from embedded target snapshots.
- Fail closed for unsupported table details that are not rendered yet, such as
  indexes, comments, RLS, exclusions, policies, triggers, and rename metadata.
- Add fixture tests for supported additive changes and unsupported/destructive
  changes.
- Keep generated metadata stable and parseable after manual review.

Acceptance criteria:

- `gosqlkit migrate create init_schema` works in an empty migration directory.
- `gosqlkit migrate create add_column` works after a baseline when the change
  is one of the supported additive operations.
- Unsupported modifications and removals return explicit planner errors and do
  not write partial migration files.
- `gosqlkit migrate check` catches malformed metadata, goose annotation
  mistakes, invalid filenames, target snapshot ID mismatches, and broken
  adjacent lineage.

### Slice 2: Structured Planner IR

Goal: move from statement-first planning to a richer change model that can
support review, risk reporting, and multiple file renderers.

Scope:

- Expand `internal/migrate/plan.Change` with object kind, stable object key,
  operation, dependencies, reversibility, risk flags, and optional statement
  groups.
- Represent create, drop, rename, alter, and replace operations explicitly.
- Keep dialect-specific SQL rendering under `internal/dialects/<dialect>/plan`.
- Keep runner-specific file layout under `internal/migrate/<runner>`.
- Preserve stable ordering across namespaces, extensions, enum/type objects,
  tables, table-attached objects, comments, policies, triggers, and indexes.

Acceptance criteria:

- Existing v0 generated files remain valid.
- The planner can produce a machine-readable plan summary without rendering a
  migration file.
- Goose rendering consumes the structured plan rather than ad hoc SQL slices.
- Unsupported operations carry enough context to explain the exact blocked
  object.

### Slice 3: Safe Additive PostgreSQL Coverage

Goal: expand automatic diffs to common additive changes that are clearly
reversible or fail-safe.

Scope:

- Add standalone indexes for new and existing tables.
- Add table and column comments.
- Add check, unique, foreign-key, primary-key, and exclusion constraints where
  PostgreSQL can apply them directly.
- Add RLS enable/force state and new policies.
- Add roles, sequences, domains, composite types, functions, triggers, views,
  and materialized views where the operation is create-only.
- Generate best-effort `Down` SQL for simple creates and renames.

Acceptance criteria:

- New-table migrations do not omit any attached object present in the snapshot.
- Additive changes are split into dependency-safe statements, following the
  Atlas pattern of table creation, separate indexes/comments, then attached
  policy/trigger style objects.
- Fixture tests cover each supported object type.
- Unsupported variants still fail closed.

### Slice 4: Destructive-Change Guardrails

Goal: make risky changes visible and blocked by default.

Scope:

- Detect table, column, enum value, constraint, index, policy, trigger, view,
  function, sequence, domain, and role removals.
- Detect column type/default/nullability/generated/identity changes.
- Detect object replacements that require drop-and-recreate planning.
- Mark risk flags such as destructive, data-loss, lock-heavy,
  non-transactional, requires-backfill, and manual-review.
- Add an explicit CLI override for destructive changes after the review model
  is in place.

Acceptance criteria:

- Destructive changes fail by default with actionable diagnostics.
- The planner reports every risky object before refusing to write files.
- Explicit override behaviour is covered by tests and never applies to unknown
  or unsupported changes silently.

### Slice 5: Rename-Aware Diffing

Goal: use existing `previousName` metadata to plan renames instead of
drop/create pairs.

Scope:

- Support table, column, constraint, index, enum/type, sequence, view,
  function, trigger, policy, and role renames where PostgreSQL has a direct
  operation.
- Validate rename metadata against previous snapshot object keys.
- Reject ambiguous rename-plus-alter combinations until semantic planning
  supports them.

Acceptance criteria:

- Rename migrations are reversible where PostgreSQL supports reversal.
- Missing or mismatched rename metadata fails with a clear error.
- Renames happen before dependent alter operations.

### Slice 6: Sandbox Replay and Drift Check

Goal: validate that committed migrations still produce the desired snapshot.

Scope:

- Add PostgreSQL connection plumbing for local/dev URLs.
- Add sandbox replay that applies committed migrations to a temporary or
  configured PostgreSQL database.
- Add PostgreSQL introspection to produce a snapshot-compatible model.
- Compare replayed database state to the latest embedded target snapshot.
- Add `gosqlkit migrate check --sandbox-url ...` and later
  `gosqlkit drift check --url ...`.

Acceptance criteria:

- Replay catches broken SQL, manual migration drift, and stale metadata.
- Introspection output is deterministic.
- Credentials and tokens are never written to generated files or logs.

### Slice 7: Auth, Apply, and Runner Expansion

Goal: make migration workflows practical in CI and managed database
environments without turning `gosqlkit` into the runtime database layer.

Scope:

- Add provider-neutral database auth interfaces.
- Support password, environment URL, custom token command, Azure Entra,
  AWS IAM, and GCP IAM token flows in the database tooling layer.
- Add `gosqlkit migrate apply --url ...` as a wrapper around generated
  migration files.
- Add a `golang-migrate` renderer from the same structured plan.
- Add machine-readable command output and quiet mode for CI.

Acceptance criteria:

- Token material is acquired just before connection and redacted from
  diagnostics.
- `migrate apply` remains optional; generated SQL stays usable with goose.
- Runner renderers do not change planner behaviour.

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
- Should the initial implementation support PostgreSQL only, or should the
  core IR be introduced with a minimal SQLite planner to prove the boundary?
