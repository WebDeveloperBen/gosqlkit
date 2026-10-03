# Design

## Context

SQLite schema model types already serialize `previousName`; table and column rename planning exists, and view/trigger planners support annotated replacements. The public SQLite builders do not expose those fields, and index planning currently treats a renamed index as unrelated create/drop changes. The renderer currently emits CREATE TABLE, CREATE INDEX, and CREATE VIEW without IF NOT EXISTS options. See `proposal.md` for motivation.

## Goals / Non-Goals

**Goals:**
- Make SQLite-native create modifiers and the existing rename metadata authorable in the public DSL.
- Make rename planning consume supported metadata deterministically and fail closed on mismatched or unsafe annotations.
- Keep creation idempotency flags from being treated as structural changes to existing database objects.

**Non-Goals:**
- Change SQLite catalog tooling or live-database workflows tracked by `sqlite-tooling`.
- Add rename support for table constraints or introduce PostgreSQL-only DDL options.
- Turn a rename plus unrelated schema change into one migration when the current planner cannot safely compose it.

## Decisions

1. **Use `PreviousName` builder methods consistently.** Mirror the existing PostgreSQL object-builder naming convention and write the supplied name into the serialized model. Support table, column, index, view, and trigger models, which are the SQLite object families identified by the existing roadmap.

2. **Store `IF NOT EXISTS` on the SQLite model objects.** Add explicit table, index, and view options so SQL rendering and snapshots preserve the declaration. Treat these flags as create-time rendering options, not database structure: changing only the flag in a previous/current snapshot produces no migration plan for an existing object.

3. **Plan index rename as drop then create.** SQLite has no `ALTER INDEX ... RENAME`; emit the old-index drop and new-index create with reverse statements. Permit only an unchanged index definition under rename metadata; reject mismatched old names and rename-plus-definition changes.

4. **Retain current planner semantics for table, column, view, and trigger renames.** Add the DSL setters and explicit annotation validation where missing. Preserve existing fail-closed behavior for table rename combined with schema changes and extend consistent mismatch handling to columns and object-level rename planners.

5. **Validate behavior at public and planner boundaries.** Extend the SQLite example/golden for the create modifiers and add tests that build public declarations, verify serialized fields, inspect rendered clauses, and exercise rename plans and invalid transitions.

## Risks / Trade-offs

- `IF NOT EXISTS` makes initial schema creation tolerant of already-existing objects but does not reconcile their definitions; documentation and tests must preserve that standard SQLite meaning.
- Index rename is not a native SQLite operation, so drop/create briefly removes the index and should remain explicitly represented in plan output.
- A wrong `PreviousName` can otherwise look like a new object plus a drop; validation must reject it before emitting a plan that could discard user data or dependencies.
