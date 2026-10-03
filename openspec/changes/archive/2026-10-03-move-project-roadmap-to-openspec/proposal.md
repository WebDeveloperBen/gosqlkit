# Proposal

## Why

The repository's implementation status is split across `FEATURES.md`, `SQLITE.md`, and `AGENTS.md`, and several SQLite checklist sections contradict the implementation and tests. Establishing OpenSpec capability specifications and change task lists as the status source of truth will let maintainers distinguish verified existing behavior from unfinished work and resume SQLite tooling from an accurate backlog.

## What Changes

- Add durable OpenSpec capability specifications for the PostgreSQL and SQLite schema workflows, grounded in current source, tests, examples, and documented dialect constraints.
- Represent implementation already present as the existing baseline; keep incomplete SQLite capabilities explicit and track implementation work in a focused change with checkable tasks.
- Move the six OpenSpec workflow commands and six OpenSpec skills from `.claude/commands/opsx/` and `.claude/skills/openspec-*/` into `.agents/commands/opsx/` and `.agents/skills/`, preserving their relative names and content. Tailor the moved `openspec-propose` skill to this project. Remove the old OpenSpec command and skill copies from `.claude`; leave unrelated Claude settings untouched.
- Remove duplicated status checklists from `FEATURES.md` and `SQLITE.md` after migrating their information; retain or replace those documents with concise pointers where their product context remains useful.
- Update `AGENTS.md` status guidance and `README.md` feature claims to point at the canonical OpenSpec status without removing architecture or user-facing documentation that remains useful.
- Preserve `SPEC.md` and `MIGRATIONS.md` as decision records unless specific sections are only duplicated status and can be safely replaced with references.

## Capabilities

### New Capabilities

- `postgres-schema-workflow`: The implemented PostgreSQL schema declaration, snapshot, rendering, migration, inspection, drift, replay, and apply behavior, with unsupported cases and safety boundaries.
- `sqlite-schema-workflow`: The SQLite-supported schema and migration-planning behavior, SQLite-specific limitations, and requirements for remaining introspection, drift, replay, apply, and integration work.
- `project-status-tracking`: Repository conventions for keeping durable capability requirements in OpenSpec specs, active implementation work in change tasks, and product/architecture documentation free of competing status boards.

### Modified Capabilities

None. The repository currently has no published OpenSpec capability specs.

## Impact

- OpenSpec: `openspec/specs/` and an active SQLite tooling change will become the canonical capability/backlog representation.
- Agent assets: `.agents/commands/opsx/`, `.agents/skills/openspec-*/`, and removal of the corresponding OpenSpec assets from `.claude/`; `AGENTS.md`, `FEATURES.md`, `SQLITE.md`, and selected feature-summary text in `README.md`.
- No application code or runtime behavior changes are in scope for the status migration itself. SQLite implementation remains a separate, explicitly tracked change.