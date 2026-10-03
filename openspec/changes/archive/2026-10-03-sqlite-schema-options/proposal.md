# Proposal

## Why

The SQLite planner already understands `previousName` metadata, but the public DSL does not expose builders to author it, and SQLite-native `IF NOT EXISTS` creation clauses are not modeled. These gaps are separate from database tooling and would otherwise disappear when the duplicate SQLite checklist is removed.

## What Changes

- Add public SQLite DSL options to render `IF NOT EXISTS` for tables, indexes, and views.
- Expose rename metadata builders for supported SQLite objects already handled by the schema model and planner.
- Add renderer, snapshot, and planner tests for the new public declaration paths.

## Capabilities

### New Capabilities
- None. This change extends `sqlite-schema-workflow`.

### Modified Capabilities
- `sqlite-schema-workflow`: add creation modifiers and authorable rename metadata to the SQLite schema workflow.

## Impact

- Public SQLite builders, SQLite schema model/rendering, migration-plan inputs, examples/goldens, and tests.
- No changes to the SQLite tooling work tracked by `sqlite-tooling`.
