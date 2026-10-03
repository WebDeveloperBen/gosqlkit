# Spec Delta

## ADDED Requirements

### Requirement: SQLite creation declarations support IF NOT EXISTS
SQLite table, index, and view declarations SHALL expose an opt-in IF NOT EXISTS clause and SHALL preserve that option in generated SQL and snapshots.

#### Scenario: Render table creation with IF NOT EXISTS
- **WHEN** a table declaration enables IF NOT EXISTS
- **THEN** generated SQL SHALL use `CREATE TABLE IF NOT EXISTS`

#### Scenario: Render index and view creation with IF NOT EXISTS
- **WHEN** an index or view declaration enables IF NOT EXISTS
- **THEN** generated SQL SHALL use the corresponding SQLite `CREATE [UNIQUE] INDEX IF NOT EXISTS` or `CREATE VIEW IF NOT EXISTS` form

### Requirement: SQLite rename intent is authorable and planned for supported objects
The SQLite DSL SHALL expose previous-name annotations for tables, columns, indexes, views, and triggers; snapshots and migration planning SHALL preserve and use those annotations, with index renames represented as drop-and-create operations because SQLite has no index-rename statement.

#### Scenario: Plan a table or column rename
- **WHEN** a table or column is declared with the previous object's name and no unsupported companion changes
- **THEN** the planner SHALL emit the corresponding SQLite rename statement and reverse statement

#### Scenario: Plan an index rename
- **WHEN** an index is declared with a previous name and otherwise unchanged definition
- **THEN** the planner SHALL emit a drop of the old index and creation of the new index, with reverse SQL

#### Scenario: Plan a view or trigger rename
- **WHEN** a view or trigger is declared with a previous name
- **THEN** the planner SHALL replace the old object with the renamed declaration and include reverse SQL

#### Scenario: Reject ambiguous rename intent
- **WHEN** a rename annotation refers to no object in the prior snapshot or is combined with changes the planner cannot safely compose
- **THEN** planning SHALL return an actionable error without emitting a partial plan
