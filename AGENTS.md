# AGENTS.md

Guidance for AI agents (and humans) working in `gosqlkit`. Read this before
touching the codebase. It captures the project's intent, architecture, the
non-obvious patterns that will bite you, and the verification workflow.

This file is intentionally **not** a duplicate of the product spec. For the
full product direction, feature list, and status, read these first and treat
them as canonical:

- [SPEC.md](SPEC.md) — the ADR: goals, non-goals, scope, architecture, phases.
- [FEATURES.md](FEATURES.md) — the live feature checklist. Update checkboxes
  as things land. This is the source of truth for "what's done / what's next".
- [MIGRATIONS.md](MIGRATIONS.md) — the migration-generation decision record:
  cross-dialect planner, goose-compatible files, embedded metadata.
- [README.md](README.md) — the user-facing intro and example.

If AGENTS.md ever disagrees with SPEC.md or FEATURES.md, those win. Update
AGENTS.md to match.

---

## 1. What this project is

`gosqlkit` is a **Go-native schema DSL and deterministic schema generator**.
PostgreSQL is the first dialect; the core is dialect-neutral so other engines
can be added later. It is explicitly **not an ORM**, **not a query builder**,
and **not a migration engine** in the first pass. It owns:

```text
Go schema definitions -> deterministic snapshot JSON -> canonical dialect SQL
```

Runtime DB access stays with `sqlc` + `pgx`. Migration diffing is handled by
the planned `gosqlkit` cross-dialect migration planner. Generated migrations
should remain goose-compatible; see MIGRATIONS.md for the decision record.

### Why we're building it

The Go ecosystem has strong runtime DB tooling (`sqlc`, `pgx`, `goose`) but no
clean, free, Go-native equivalent of the **Drizzle Kit / Prisma schema
workflow**. See SPEC.md "Context" and "Alternatives considered" for the full
rationale.

---

## 2. The Drizzle inspiration (depth reference, not blueprint)

The user's explicit framing: use
[drizzle-team/drizzle-orm](https://github.com/drizzle-team/drizzle-orm) and
its `drizzle-kit` subtree as the **depth reference** for what a complete
schema-management surface looks like — **not** as an implementation blueprint.

### Borrow from Drizzle

- The breadth of PostgreSQL schema objects: tables, columns, enums, schemas,
  sequences, composite types, domains, views, materialized views, roles,
  policies, RLS, functions, triggers.
- **Structured** modelling of things that are easy to stringly-type:
  - Identity columns with sequence options (`GENERATED {ALWAYS | BY DEFAULT}
    AS IDENTITY (...)`).
  - Generated stored columns (`GENERATED ALWAYS AS (...) STORED`).
  - Index columns with method, opclass, order, nulls, partial predicate,
    `CONCURRENTLY`, `ONLY`, `WITH (...)` storage params.
  - Defaults that aren't raw strings: typed helpers for strings, ints, bools,
    JSON, arrays, dates, timestamps.
  - Sequences with increment/min/max/start/cache/cycle/ownership.
- The serializer pattern: schema is collected into a **structured snapshot**
  before any diffing, and the snapshot is the diff input — not the rendered
  SQL text. `gosqlkit` already does this (`internal/dialects/pg/pgschema`'s
  `JSON` function); future diff work builds on it.
- Stable object identity and metadata maps for rename detection and
  destructive-change guards (planned, see FEATURES.md "Serialisation and Diff
  Readiness").

### Do **not** borrow from Drizzle

- The TypeScript generics / builder-brand typing. We use plain Go structs and
  fluent methods.
- The runtime ORM and query builder. Out of scope.
- Runtime column mapping (`mapFromDriverValue` etc.). That's `sqlc`'s job.
- Process-exit-on-error inside library code. Drizzle's serializer does this;
  we return `error` and let the CLI decide.

### Where to look in Drizzle when designing a new PG feature

- `drizzle-orm/src/pg-core/columns/*.ts` — one file per scalar type, plus
  `int.common.ts` for the identity-builder base, `custom.ts` for the
  custom-type escape hatch, `vector_extension/` and `postgis_extension/` for
  extension-provided types.
- `drizzle-orm/src/pg-core/{indexes,foreign-keys,unique-constraint,primary-keys,checks,policies,roles,sequence,view,view-common,view-base}.ts`
  — the constraint/object builder shapes.
- `drizzle-kit/src/serializer/pgSerializer.ts` — how Drizzle collects
  declarations into its snapshot, including identity sequence defaults and
  generated columns. This is the best single reference for "what fields does a
  diff-ready snapshot need?".
- `drizzle-kit/src/sqlgenerator.ts` — how snapshot statements become SQL.

---

## 3. Repository layout and layering

```text
cmd/gosqlkit/            process entrypoint only; calls cli.Run, maps exit codes
internal/cli/            Kong command structs, help, arg parsing, exit-code mapping
internal/app/            use cases: plain Go option/result types, no Kong dep
kit/                     dialect-neutral provider registry, aliases, capabilities
pg/                      PUBLIC PostgreSQL DSL (user-facing import path)
internal/dialects/pg/    PostgreSQL internal machinery, grouped:
  pgschema/              PG schema envelope: namespaces, extensions, enums,
                         sequences, composite types, domains + ast.Tables.
                         Also owns the snapshot JSON function — the model types
                         carry JSON tags directly, so the snapshot IS the model
                         serialised. No separate snapshot conversion layer.
  render/                PG SQL renderer + all validation
internal/ast/            shared dialect-neutral schema core: Table, Column,
                         constraints, indexes. Types carry JSON tags so they
                         serve as both the in-memory model and the snapshot.
internal/version/        build info
examples/basic/          end-to-end example feeding sqlc
```

### Why the dialect grouping exists

All PostgreSQL-specific internal code lives under `internal/dialects/pg/`.
When a new dialect is added (mysql, sqlite, mssql, etc.), it gets its own
directory under `internal/dialects/` with the same sub-structure
(`schema/`, `snapshot/`, `render/`). This keeps `internal/` from filling up
with siblings like `internal/mysqlschema/`, `internal/mysqlsnapshot/`, etc.
and makes the dialect boundary explicit.

The **shared, dialect-neutral** packages stay at the top of `internal/`:
- `internal/ast/` — the schema core types common to all dialects (these
  carry JSON tags so they double as the snapshot types).

The **public DSL** stays at the top level (`pg/`, future `mysql/`, etc.)
because that is the user-facing import path. It can't move to `internal/`.

### Layering rules (enforced by review, not yet by lint)

- `cmd/gosqlkit` is only the process boundary: translate panics/exit codes.
- `internal/cli` owns Kong structs and exit mapping. No business logic.
- `internal/app` owns use cases with plain Go types. No Kong import.
- `kit` owns the dialect-neutral registry.
- Dialect packages (`pg`, future `mysql`, etc.) own their DSL, registry,
  renderer selection, and snapshot dialect marker.
- `internal/ast`, `pg`, `kit`, `internal/dialects/`
  **never import CLI packages**.
- A new CLI command becomes an `internal/app` function first, then a thin
  `internal/cli` adapter. See `generate.go` / `snapshot.go` for the shape.

### Adding a new dialect

1. Create `internal/dialects/<dialect>/` with `schema/` and `render/`
   sub-packages mirroring `internal/dialects/pg/`. The schema package owns
   both the model types (with JSON tags) and the `JSON()` snapshot function.
2. Create a top-level `<dialect>/` package for the public DSL (must stay
   outside `internal/` — it's the user import path).
3. Register the provider with `kit` via `init()` in the public DSL package.
4. Implement `kit.Provider` (`Dialect()`, `RenderSQL()`, `SnapshotJSON()`).
5. Add the dialect to `internal/app/generate.go`'s dialect default if needed.
6. Add an example schema and golden tests.

---

## 4. Key design patterns (the non-obvious bits)

These are the patterns that will trip you up if you don't know them.

### 4.1 DSL builders return typed `*Def` structs; `Element.apply` mutates the table

A table is built from `Element`s. Each element (`*Column`, `*CheckDef`,
`*IndexDef`, `*PrimaryKeyDef`, `*UniqueConstraintDef`, `*ForeignKeyDef`,
`*ExclusionDef`) implements `apply(table *ast.Table)` and appends itself.

```go
type Element interface { apply(table *ast.Table) }
```

The `apply` methods are unexported so users can't call them directly; the only
way to attach an element is to pass it to `pg.Table` / `pg.TableInSchema`.

### 4.2 The registry stores `*ast.Table` (pointers), not values

`pg.Definition` holds `*ast.Table` and the registry stores `*ast.Table`. This
is **required** because some builder methods mutate the table **after**
construction — notably `(*Definition).Comment(text)` and the index
auto-naming in `(*IndexDef).apply`. If the registry stored values, those
mutations would be lost.

When you add a new table-level post-construction builder method, this pointer
pattern is why it works. Don't switch back to value storage.

### 4.3 Domains are stored as pointers in the registry; enums/sequences/composite types are not

`pgschema.Domain` is stored as `*pgschema.Domain` in the `pg` registry because
the `Domain` DSL supports post-construction mutation (`.Default()`,
`.NotNull()`, `.Check()`). Enums, sequences, and composite types are
constructed in one shot and stored as values.

When adding a new schema object DSL:
- If it has post-construction builder methods -> store `*T` in the registry
  and copy out in `Schema()`.
- If it's constructed once -> store `T`.

### 4.4 Validation lives in the renderer, not at DSL construction time

DSL constructors only panic for programmer errors that have nothing to do
with SQL correctness (e.g. `varchar` length <= 0, array dimensions < 1,
wrong arg count). All SQL-level validation — identifier shape, unknown
columns, FK actions, identity-on-integer-type, generated-vs-default
conflicts, duplicate names, deferrable `INITIALLY` values, exclusion
operators — happens in `internal/dialects/pg/render/postgres.go` `validateSchema` and
returns `error`.

This keeps the DSL cheap to build and concentrates correctness logic in one
place that both `generate` and `snapshot` run (snapshot calls
`render.Postgres` first to validate before serializing).

**When adding a feature: add validation to `validateSchema`, not to the
builder.**

### 4.5 Deterministic ordering is mandatory

Every rendered output and every snapshot must be byte-for-byte stable across
runs regardless of map iteration order or input order. Conventions:
- `sort.SliceStable` by name (or by `schema.name` qualified key) everywhere.
- Tables are topologically sorted by FK dependency (`orderTables`), with
  alphabetical tie-break via the initial `sort`.
- Index `WITH (...)` storage params are rendered with sorted keys.
- Snapshot JSON is `json.MarshalIndent` with sorted table/constraint/index
  lists; maps in the AST (e.g. `Index.With`) are copied into sorted output.

If you add a new object collection, sort it before rendering and before
snapshotting, and add a golden test.

### 4.6 The snapshot is the model serialised, not a separate type

The AST and pgschema model types carry JSON tags directly. The snapshot IS
`json.MarshalIndent` of the model — there is no separate snapshot conversion
layer. This is deliberate: when you add a schema field, you add it to the
model once (with a JSON tag) and add rendering once. You do NOT need to
update a second set of snapshot types.

`pgschema.JSON()` handles deterministic sorting before marshalling and wraps
the output with `version` and `dialect` metadata. The version
(`pgschema.SnapshotVersion`) bumps only when the JSON shape changes in a way
that would break a diff consumer.

The snapshot is what future migration diffing will consume.

### 4.7 SQL generation runs a generated Go program, not reflection

`internal/app/generate.go` doesn't load the user's schema package via
reflection. It writes a tiny `package main` into the OS temp dir, blank-imports
the user's schema package(s) (which triggers `pg` registry side-effects via
`var` declarations), calls `kit.RenderSQL`/`kit.SnapshotJSON`, and `go run`s
it with `cmd.Dir` set to the module root so imports resolve. The user's
project directory is not touched. This is why schema declarations are
package-level `var`s and why the registry uses `init()` + `sync.Mutex`.

The generated program's import path for `kit` is discovered dynamically via
`go list -m` (not hardcoded), so the repo can be renamed or forked without
breaking generation.

The schema source paths come from `gosqlkit.yaml` (the config file). The CLI
discovers the config by searching the current directory and parents, reads
the schema paths from it, resolves them to Go import paths via `go list`, and
generates a program that blank-imports all of them. Multiple schema paths are
supported — the generated program imports all packages, flattening their
declarations into one registry.

Don't try to replace this with `plugin` or reflection — the `go run` approach
is deliberate, cross-platform, and keeps the user's build tags intact.

### 4.8 Identifier rules

`internal/dialects/pg/render/postgres.go` `identifierPattern` = `^[a-z_][a-z0-9_]*$`.
Identifiers (tables, columns, constraints, indexes, sequences, enums, etc.)
must match this. Extension names allow hyphens via `validateExtensionName`.
Qualified names (`schema.name`) are parsed by `parseQualifiedIdentifier` and
rendered by `renderQualifiedName` / `renderReferencedTable`. The `public`
schema is the implicit default and is normalised via `qualifiedName` /
`tableSchema` for keys.

---

## 5. How to add a schema feature (end-to-end)

Use this checklist. Example: adding a new column type, constraint, or schema
object.

1. **Model** — add the struct to `internal/ast/schema.go` (if shared across
   dialects) or `internal/dialects/pg/pgschema/schema.go` (if PG-specific).
   Add a JSON tag so it serialises into the snapshot automatically. Add it to
   the containing struct (`Table`, `Schema`, etc.).
2. **DSL** — add a constructor + fluent methods in `pg/` (`column.go`,
   `table.go`, or `schema.go`). Return a typed `*Def`. Implement `apply` if
   it's a table element. Register PG-specific objects in `pg/registry.go`.
3. **Render** — add a `render*` function in `internal/dialects/pg/render/postgres.go`
   and call it from `Postgres` in the right position (namespaces -> extensions ->
   enums -> composite types -> domains -> sequences -> tables -> indexes ->
   comments). Add **validation** to `validateSchema`.
4. **Example** — exercise the feature in `examples/basic/schema/schema.go`.
5. **Golden** — regenerate:
   ```bash
   task generate
   task snapshot
   ```
   Then copy the canonical SQL into `internal/dialects/pg/render/testdata/postgres.golden.sql`
   if `TestPostgresRender` should cover it (the test builds the schema
   directly via `internal/ast`/`internal/dialects/pg/pgschema`, so update its
   input too).
6. **FEATURES.md** — flip the relevant `[ ]` to `[x]` and add tests to the
   "Testing Requirements" section if applicable.
7. **Verify** — run the full suite (section 7).

### Where validation goes (recap)

| Kind                              | Where                                  | Mechanism        |
|-----------------------------------|----------------------------------------|------------------|
| Programmer error (bad DSL args)   | `pg/` constructor                      | `panic`          |
| SQL correctness (identifiers, refs, types, conflicts) | `internal/dialects/pg/render/postgres.go` `validateSchema` | `error` return |

Never panic from the renderer for schema-level issues; always return an
error so the CLI can format it.

---

## 6. The example and golden files

`examples/basic/` is the canonical end-to-end example. It must always:

- Declare a realistic schema exercising the current feature set.
- Generate SQL that `sqlc` can consume (`examples/basic/sqlc.yaml`).
- Have a matching snapshot JSON.
- Be regeneratable with `task generate` and `task snapshot`.

Golden files:
- `examples/basic/db/schema.generated.sql` — checked in. Regenerate, don't
  hand-edit.
- `examples/basic/db/schema.snapshot.json` — checked in. Regenerate, don't
  hand-edit.
- `internal/dialects/pg/render/testdata/postgres.golden.sql` — the self-contained golden
  for `TestPostgresRender`. This is **not** auto-copied from the example; it
  is the expected output for the schema built directly in
  `internal/dialects/pg/render/postgres_test.go`. When you change rendering, update both
  the test schema and this golden (run the test, copy the got output).

The render test is decoupled from the example on purpose: the example is a
user-facing integration test, the render test is a focused unit test of the
renderer. Don't re-couple them.

---

## 7. Verification commands (Taskfile)

`Taskfile.yml` is the source of truth. Key tasks:

```bash
task                  # = task verify (the full local gate)
task test             # go test ./...
task build            # build the CLI into bin/
task generate         # regenerate example SQL (reads gosqlkit.yaml)
task generate:check   # fail if committed SQL is stale (CI gate)
task snapshot         # regenerate example snapshot JSON (reads gosqlkit.yaml)
task snapshot:check   # fail if committed snapshot is stale (CI gate)
task fmt              # gofumpt -w .
task fmt:check        # fail if files need formatting
task lint             # golangci-lint run ./...
task vuln             # govulncheck ./...
task sqlc:check       # run sqlc generate in a temp copy of examples/basic
task verify:cli       # smoke-test the CLI (--help, version, generate, snapshot)
task modernize        # apply Go modernization fixes
task fix:fieldalignment  # reorder struct fields to reduce padding
```

`task verify` runs: fmt:check -> build -> test -> generate:check ->
snapshot:check -> sqlc:check -> verify:cli -> lint -> vuln.

**Before declaring any task done, run `task verify` and ensure it passes
locally.** If `sqlc` or `golangci-lint` aren't installed, the relevant tasks
will tell you; install them rather than skipping.

`lefthook.yml` wires a subset of this (modernize, gofumpt, build, lint, test,
verify:cli) into a pre-commit hook. Run `task setup` once to install it.

---

## 8. Code conventions

- **Go version**: see `go.mod` (currently 1.26.4). Don't lower it.
- **Formatting**: `gofumpt` (stricter than `gofmt`). Run `task fmt` before
  committing. `gofumpt` is the toolchain entry in `go.mod`.
- **No code comments** unless explicitly requested. The codebase is
  intentionally comment-light; types and function names carry the meaning.
  Doc comments on package entry points (`doc.go` files) and exported symbols
  are fine and expected by `godoclint`.
- **External test packages**: tests use the `_test` suffix
  (`package render_test`, `package pg_test`, etc.) so they exercise the
  public API. Keep doing this.
- **Errors**: wrap with `%w` for chaining; use `fmt.Errorf("context: %w",
  err)`. The renderer wraps column/table context into errors consistently —
  follow that shape.
- **No `panic` in library code** for schema-level issues. Only panic in DSL
  constructors for programmer errors (section 5).
- **No new dependencies without thought**. Runtime dependencies are limited to
  CLI/config tooling plus the internal PostgreSQL tooling connection path
  (`pgx`) used by sandbox replay. The public schema DSL must not pull in a
  database driver.
- **gosec**: existing `#nosec` directives have justifications. Don't strip
  them; don't add new suppressions without a comment.

---

## 9. AI ways of working in this repo

These are the working rules that make AI-assisted work on this repo
predictable and reviewable.

### 9.1 Always read the context docs first

Before any change, read [SPEC.md](SPEC.md), [FEATURES.md](FEATURES.md), and
[README.md](README.md). They are short and canonical. AGENTS.md is the
**how**, not the **what**.

### 9.2 Pick the right track

FEATURES.md "Recommended Build Order" and the open `[ ]` items are the
backlog. When asked to "continue" or "pick up implementation", the remaining
tracks are roughly:

1. **Schema DSL coverage** — missing scalar types, arrays, identity,
   generated columns, sequences, composite types, domains, safe default
   helpers, deferrable/exclusion constraints, comments, named index helpers.
   (Largely landed; check FEATURES.md for residual `[ ]`.)
2. **Snapshot metadata + diff readiness** — stable snapshot IDs, previous
   snapshot ID, schema/table/column/view/role/function/trigger/policy metadata maps, stable
   object keys, rename annotations, squashed/normalised diff representation.
   Prerequisite for high-quality migration diffing. Mostly landed; rename-aware
   diffing (Slice 5) and column-modification / replacement planning are the
   remaining open pieces.
3. **Database connectivity + introspection + diff** — cross-dialect migration
   IR, dialect planners, sandbox validation, drift checks, and provider-pluggable
   token auth for Azure/AWS/GCP. Largest track (Slices 6 and 7).
4. **Advanced PG objects** — partitioned tables, grants, and other remaining PostgreSQL objects.

When direction is ambiguous, ask the user which track rather than guessing.
A wrong track wastes more time than a quick clarifying question.

### 9.3 One model, one render pass

A feature touches: model (with JSON tag) -> DSL -> render -> validate ->
example -> golden -> FEATURES.md. The model is the single source of truth —
the snapshot is just `json.Marshal` of the model with deterministic sorting,
so there is no separate snapshot layer to keep in sync. Skipping the render
step means the feature exists in the model/snapshot but never reaches SQL.

### 9.4 Regenerate, don't hand-edit golden files

After any DSL/renderer change:
```bash
task generate && task snapshot
```
Then update `internal/dialects/pg/render/testdata/postgres.golden.sql` and the test schema
in `internal/dialects/pg/render/postgres_test.go` together. Never edit the golden SQL by
hand to "make the test pass" — that defeats the point.

### 9.5 Keep the CLI layering

A new CLI command means:
1. An `internal/app` function with plain options/result structs (no Kong).
2. A thin `internal/cli` command struct that calls it and maps errors via
   `Exit(code, err)`.
3. A line in `CLI` in `internal/cli/root.go`.

Don't put business logic in `internal/cli`. Don't import `internal/app` from
`pg` or `internal/ast`.

### 9.6 Verify before stopping

Run `task verify` before declaring a task complete. If something can't be
verified (e.g. `sqlc` not installed), say so explicitly and run what you can
(`task test`, `task build`, `task fmt:check`, `task lint`).

If lint or vuln flags something, fix it — don't suppress. The lint config
(`.golangci.yml`) is deliberately tight; the repo should stay green.

### 9.7 Don't commit unless asked

Never commit, amend, push, or open a PR unless the user explicitly asks.
Inspect `git status` / `git diff` first if they do ask, and stage only
intended files. The repo has a pre-commit hook that will reformat and
verify — let it run.

### 9.8 Keep the diff reviewable

- Prefer small, focused edits over big rewrites.
- Don't reformat code you didn't touch. `task fmt` will format everything;
  that's fine, but don't bundle a wholesale reformat with a feature change.
- Don't add comments explaining "what" — the code says what. Doc comments
  explaining "why" on exported symbols are welcome.
- Don't introduce emojis in code or docs unless asked.

### 9.9 When blocked

- **Ambiguous product direction** -> ask the user (question tool).
- **Ambiguous technical choice with a clear "right" answer in this repo** ->
  pick the one that matches existing patterns and note the decision in the
  PR description or a one-line code doc.
- **Lint won't pass and the fix is unclear** -> read `.golangci.yml` and the
  specific linter docs; don't add `//nolint` without a real reason and an
  explanation (the `nolintlint` linter requires both).

### 9.10 Updating the docs

- **FEATURES.md** — flip checkboxes as features land. Add new rows under the
  right section. This is the live status board.
- **SPEC.md** — only when the decision changes. This is an ADR, not a
  changelog.
- **README.md** — user-facing examples and the feature summary. Keep the
  "Current Features" list in sync with FEATURES.md's `[x]` items.
- **AGENTS.md** — when a pattern or working rule changes, or when a new
  non-obvious gotcha is discovered. Don't bloat it; if it grows past ~600
  lines, split or trim.

---

## 10. Common pitfalls (gotchas)

- **Storing values instead of pointers in the `pg` registry** breaks
  post-construction builder methods (`Comment`, index auto-naming, domain
  mutators). See section 4.2 / 4.3.
- **Validating in the builder** instead of `validateSchema` splits
  correctness logic and skips validation for direct-AST users (the renderer
  test, future introspection). See section 4.4.
- **Forgetting to sort** a new collection makes output non-deterministic and
  breaks golden tests intermittently. See section 4.5.
- **Forgetting to add a JSON tag** when adding a model field means the
  feature renders to SQL but is invisible to the snapshot (and therefore to
  future diffs). The model is the single source of truth — see section 4.6.
- **Editing golden files by hand** hides regressions. Regenerate. See 9.4.
- **Coupling the render test to the example** made the test brittle; they're
  now decoupled. Keep them decoupled. See section 6.
- **Importing CLI packages from DSL/renderer/ast** breaks layering. Don't.
- **Adding a database driver to the public DSL/runtime path** violates the "no
  ORM, no runtime" scope. Internal tooling may use `pgx` for sandbox replay
  and future introspection, but application runtime access stays with the
  user's `sqlc`/`pgx` layer.

---

## 11. Current state at a glance

See [FEATURES.md](FEATURES.md) for the canonical checklist. As of the last
update of this file:

- **Landed**: Phase 1 (schema DSL + SQL generation) and Phase 2 (sqlc
  compatibility) are complete. The DSL covers tables, columns (all common PG
  scalar types + arrays + identity + generated), constraints (PK, unique
  with `NULLS NOT DISTINCT`, FK with deferrable, checks, exclusion), indexes
  (full advanced surface), schemas, extensions, enums, sequences, composite
  types, domains, roles, functions, triggers, RLS policies, views, materialized views, comments, safe
  default helpers, custom-type escape hatch.
  CLI has `generate`, `snapshot` (each with `--out`, `--check`, and `--prev`
  for snapshot), `version`, `migrate create` (with `--empty`, `--no-down`,
  `--allow-destructive`), `migrate plan` (with `--json`), `migrate check`.
  Snapshot JSON is versioned, dialect-tagged, includes stable snapshot IDs
  (SHA-256), metadata maps (schema/table/column/view/role/function/trigger/policy), and rename
  annotations (previousName on all objects).
  Migration planner emits structured changes with per-change risk flags
  (`destructive`, `data-loss`, `lock-heavy`, `manual-review`,
  `requires-ddl-review`) and best-effort reverse SQL. Destructive changes
  (drops, RLS disables, comment removals, enum value removals, and column type
  changes) fail by default and require `--allow-destructive` to author when
  executable SQL exists. Column type, default, nullability, generated
  expression, and identity changes are planned as structured `ALTER COLUMN`
  changes with risk metadata; inline column constraint/reference mutations
  still fail closed. Enum value removals are detected as manual-review
  replacement changes and intentionally emit no automatic SQL. Renames (table,
  column, constraint, index, enum/type, sequence, view, materialized view,
  function, trigger, policy, role, schema) are planned as `ALTER ... RENAME TO`
  with a reverse `RENAME TO old_name`; extension rename metadata is detected
  as a manual-review replacement because PostgreSQL cannot rename extensions.
  Mismatched `previousName` and rename-plus-alter combinations fail closed.
  PostgreSQL sandbox replay is implemented: `migrate check --sandbox-url` opens a
  tooling-only `pgx` connection, replays committed goose `Up` sections into a
  disposable database, and verifies the latest embedded target snapshot ID
  matches the current generated schema snapshot, then introspects the replayed
  database and compares it to the embedded target snapshot. PostgreSQL drift
  checking is implemented with `drift check --url`; the introspector covers
  namespaces, extensions, roles, enums, composite types, domains, standalone
  sequences, functions, tables, columns, comments, RLS flags, table
  constraints, standalone indexes, policies, triggers, views, and materialized
  views. Drift projection normalises rename metadata, non-persistent index
  flags, identity-backed sequences, extension-owned objects, dependency hints,
  and PostgreSQL defaults.
- **Next tracks**: auth/apply/runner expansion (Slice 7), then advanced PG
  objects (partitioning, grants).
  See FEATURES.md for the open `[ ]` items.

When you change the state, update FEATURES.md first, then this section.

---

## 12. Quick reference: file responsibilities

| File / dir                              | Owns                                              |
|-----------------------------------------|---------------------------------------------------|
| `cmd/gosqlkit/main.go`                  | process entry, panic->exit code                   |
| `internal/cli/root.go`                  | Kong CLI struct, `Run`, exit mapping              |
| `internal/cli/{generate,snapshot,version}.go` | command structs calling `internal/app`     |
| `internal/app/generate.go`              | `Generate`/`Snapshot` use cases, `go run` harness |
| `internal/app/config.go`               | `gosqlkit.yaml` config parsing and discovery     |
| `kit/registry.go`                       | dialect-neutral provider registry                 |
| `pg/registry.go`                        | PG provider + in-memory schema registry           |
| `pg/column.go`                          | column constructors + column builder methods      |
| `pg/table.go`                           | `Table`, table elements, constraints, exclusion   |
| `pg/schema.go`                          | namespaces, extensions, enums, sequences, types, domains |
| `pg/defaults.go`                        | safe default helpers (string/int/bool/json/array/date) |
| `internal/ast/schema.go`                | shared schema core structs (carry JSON tags)      |
| `internal/dialects/pg/pgschema/schema.go` | PG schema envelope + snapshot JSON function     |
| `internal/dialects/pg/render/postgres.go` | PG SQL rendering + all validation               |
| `examples/basic/`                       | end-to-end example + sqlc config                  |
| `internal/dialects/pg/render/testdata/` | renderer golden files                             |
