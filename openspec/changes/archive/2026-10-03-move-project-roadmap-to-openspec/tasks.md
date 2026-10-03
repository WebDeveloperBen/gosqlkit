# Tasks

## 1. Verify the current baseline

- [x] 1.1 Run `go run ../../cmd/gosqlkit generate --check` and `go run ../../cmd/gosqlkit snapshot --check` from `examples/sqlite`; both commands exit successfully.
- [x] 1.2 Inspect SQLite planner tests and confirm they cover create/add/drop, destructive rebuild, table/column rename, trigger recreation, and empty plans in `internal/dialects/sqlite/plan/plan_test.go`.
- [x] 1.3 Audit PostgreSQL and SQLite capability claims against their DSL, renderers, planners, tooling dispatch, tests, examples, and Taskfile; identify stale SQLite checklist marks and record remaining SQLite-native work separately from PostgreSQL-only non-equivalents.

## 2. Establish capability specifications

- [x] 2.1 Refine `postgres-schema-workflow` from the evidence audit: modeled PostgreSQL object families, deterministic validated SQL, snapshots/metadata, guarded migration planning, database tooling, and configured authentication are stated at behavior level; verified implementation references are in the audit source/test inventory.
- [x] 2.2 Refine `sqlite-schema-workflow` to separate implemented SQLite DSL/render/snapshot/planner and runner-neutral file creation from missing database tooling; record SQLite-specific semantics and exclude PostgreSQL-only server capabilities rather than copying PostgreSQL work into SQLite.
- [x] 2.3 Validate all three capability specs with `openspec validate move-project-roadmap-to-openspec --type change`; correct any requirement/scenario errors.

## 3. Create the actionable SQLite tooling backlog

- [x] 3.1 Created `sqlite-tooling` with proposal, modified SQLite tooling spec, design, and ordered task artifacts; `openspec status --change sqlite-tooling --json` reports every planning artifact done.
- [x] 3.2 Added observable SQLite tooling tasks for connection/PRAGMA setup, catalog introspection, drift projection and diagnostics, isolated sandbox replay, goose/golang-migrate apply, capability/CLI routing, and opt-in embedded integration tests.
- [x] 3.3 Confirmed `sqlite-tooling/tasks.md` contains no unfinished generation, snapshot, planner, or table-rebuild tasks; those behaviors are implemented and tested.
- [x] 3.4 Created a separate `sqlite-schema-options` change for the audited `IF NOT EXISTS` clauses and public rename-builder gaps; its ordered schema/render/planner tasks do not overlap SQLite database tooling.

## 4. Make OpenSpec the repository status source

- [x] 4.1 Moved all six OpenSpec commands to `.agents/commands/opsx/` and all six skills to `.agents/skills/`; the destination inventory contains all 12 files and the old `.claude` files are absent.
- [x] 4.2 Tailored `.agents/skills/openspec-propose/SKILL.md` to require code/test evidence, prevent duplicate capability/status tracking, distinguish implemented from planned behavior, and use OpenSpec list/status commands as canonical status.
- [x] 4.3 Remove duplicated implementation-status checklists from `FEATURES.md` and `SQLITE.md` after migration, retaining concise links and any decision context; verify both documents no longer claim a competing canonical backlog.
- [x] 4.4 Updated `AGENTS.md` to use OpenSpec specs/tasks as the canonical behavior/status source and revised `README.md` for verified PostgreSQL and SQLite support; checked that architecture guidance and examples remain present.
- [x] 4.5 Preserved `SPEC.md` and `MIGRATIONS.md` as decision records, replaced obsolete phase/status checklists with OpenSpec references, and verified no current behavior is described as future solely due to old status text.
- [x] 4.6 Verified Codex repository discovery for `.agents/skills/`, checked all six skill manifests and the available `openspec` CLI, and documented Codex `$openspec-*` entrypoints; `.agents/commands/opsx/` is identified as client-specific rather than a universal slash-command registry.

## 5. Verify the status migration

- [x] 5.1 Archived the roadmap-migration change to publish the PostgreSQL, SQLite, and project-status specs; `openspec list --specs --json` lists all three.
- [x] 5.2 Validated all three published specs and both active SQLite changes; OpenSpec listings, SQLite tasks, and repository links agree on `sqlite-tooling` and `sqlite-schema-options` as the remaining SQLite work.