# Proposal

## Why

SQLite schema generation, snapshots, and migration planning are implemented, but database-backed inspection, drift checks, sandbox replay, and migration application are not. Track this remaining work as an ordered change so the SQLite capability specification, CLI behavior, and embedded integration coverage stay aligned.

## What Changes

- Add SQLite connection and catalog introspection for supported schema objects.
- Add SQLite snapshot normalization and object-level drift comparison.
- Add SQLite sandbox replay and runner-compatible migration application with explicit foreign-key handling and migration state.
- Add opt-in embedded SQLite integration coverage and wire SQLite tooling capabilities into the app/provider.

## Capabilities

### New Capabilities
- None. This change extends the `sqlite-schema-workflow` capability being established by `move-project-roadmap-to-openspec`.

### Modified Capabilities
- `sqlite-schema-workflow`: fulfill its database tooling requirements for inspect, drift, sandbox replay, and apply.

## Impact

- SQLite provider/tooling, app capability dispatch, CLI database-backed workflows, shared migration runner integration, and integration tests.
- No changes to the already implemented SQLite DSL, SQL generation, snapshot schema, migration planner, or table-rebuild behavior.
