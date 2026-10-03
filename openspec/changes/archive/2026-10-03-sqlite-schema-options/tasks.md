# Tasks

## 1. SQLite idempotent creation options

- [x] 1.1 Add model fields and fluent DSL options for `IF NOT EXISTS` on tables, indexes, and views; verify each option appears in the generated snapshot.
- [x] 1.2 Render the option in table, unique/non-unique index, and view DDL; verify exact clauses in renderer tests and the SQLite golden.
- [x] 1.3 Verify changing only an `IF NOT EXISTS` option on an existing object does not produce a structural migration plan.

## 2. Public rename metadata and planning

- [x] 2.1 Add `PreviousName` builder methods for SQLite tables, columns, indexes, views, and triggers; verify each public declaration serializes the supplied old name.
- [x] 2.2 Make index rename metadata plan a reversible drop/create pair; verify rename-plus-definition-change and unmatched previous names fail closed.
- [x] 2.3 Validate unmatched or ambiguous previous-name annotations for tables, columns, views, and triggers before producing a migration plan; verify no partial plan is returned.
- [x] 2.4 Add public-DSL-to-planner coverage for table, column, index, view, and trigger renames, including reverse SQL and the existing fail-closed rename-plus-change boundary.

## 3. Example and verification

- [x] 3.1 Exercise the new creation options and rename metadata in the SQLite example or focused public-API tests; regenerate the SQLite SQL and snapshot goldens.
- [x] 3.2 Verify the focused SQLite renderer/planner tests and `generate --check` / `snapshot --check` pass, then update the README feature summary for the newly supported options.
