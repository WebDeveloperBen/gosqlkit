# sqlite-schema-workflow Specification

## Purpose
Defines SQLite's supported schema-as-code, snapshot, and migration behavior, preserving SQLite-specific semantics and separating implemented workflow from remaining database tooling.

## Requirements

### Requirement: SQLite declarations model SQLite-supported schema objects
The SQLite provider SHALL offer typed declarations for tables, columns, table constraints, indexes, regular and temporary views, triggers, and ordered raw SQL blocks; it SHALL NOT advertise PostgreSQL-only server objects as SQLite capabilities.

#### Scenario: Declare SQLite-affinity columns and table options
- **WHEN** a schema defines SQLite storage affinities, generated columns, AUTOINCREMENT, STRICT, or WITHOUT ROWID options
- **THEN** the model and rendered SQL SHALL preserve those declared SQLite semantics

#### Scenario: Declare constraints and indexes
- **WHEN** a schema defines supported primary, unique, foreign-key, check, expression, or partial-index declarations
- **THEN** rendered SQL SHALL preserve their columns, actions, deferral, ordering, collations, and predicates

#### Scenario: Exclude PostgreSQL-only capabilities
- **WHEN** a caller selects SQLite
- **THEN** the provider SHALL NOT advertise schemas, extensions, enums, standalone sequences, domains, composite types, materialized views, roles, RLS, policies, grants, or server-side functions

### Requirement: SQLite rendering is deterministic and validates declarations
The SQLite renderer SHALL produce canonical SQLite DDL in deterministic order and SHALL reject invalid identifiers, duplicate declarations, and invalid SQLite-specific combinations with an error.

#### Scenario: Validate STRICT and WITHOUT ROWID tables
- **WHEN** a table uses STRICT or WITHOUT ROWID with unsupported types or without required primary-key semantics
- **THEN** rendering SHALL return a contextual validation error

#### Scenario: Equivalent declarations render consistently
- **WHEN** equivalent schemas are supplied in different collection orders
- **THEN** canonical SQL ordering SHALL remain stable

### Requirement: SQLite snapshots preserve modeled SQLite schema state
The SQLite provider SHALL emit deterministic, versioned, dialect-tagged snapshots with stable identifiers and metadata for modeled tables, columns, indexes, views, triggers, and raw SQL.

#### Scenario: Stable SQLite snapshot generation
- **WHEN** the same SQLite schema is serialized repeatedly
- **THEN** its normalized snapshot content and identifier SHALL be stable

### Requirement: SQLite migration planning handles SQLite ALTER TABLE limits safely
SQLite migration planning SHALL use supported direct alterations where possible and SHALL plan a data-preserving table rebuild for supported changes requiring reconstruction; destructive changes SHALL carry risk and require explicit authorization.

#### Scenario: Rebuild preserves surviving data and dependents
- **WHEN** a supported schema change requires a table rebuild
- **THEN** the plan SHALL copy values for surviving columns and recreate affected indexes and triggers

#### Scenario: Destructive rebuild is guarded
- **WHEN** a rebuild drops columns or otherwise risks data loss
- **THEN** the plan SHALL identify the risk and migration creation SHALL fail unless destructive authoring is explicitly enabled

#### Scenario: Unsupported rename combination fails closed
- **WHEN** rename metadata is present alongside changes the planner cannot safely compose
- **THEN** the planner SHALL reject the combined plan with an actionable error

### Requirement: SQLite schema commands support generation, snapshots, and migration planning
The SQLite provider SHALL support configured schema generation and snapshot checks, migration file creation, and structured migration planning through the runner-neutral CLI.

#### Scenario: Check generated SQLite artifacts
- **WHEN** generated SQL or snapshot output differs from its checked-in target
- **THEN** the corresponding check command SHALL report the artifact as stale and exit unsuccessfully

#### Scenario: Create runner-compatible migration files
- **WHEN** a SQLite migration is created for a supported runner
- **THEN** the emitted file layout SHALL conform to the selected goose or golang-migrate format

### Requirement: SQLite database tooling supports inspection, drift, replay, and apply
SQLite tooling SHALL connect to SQLite file or in-memory databases, introspect supported schema objects into the SQLite snapshot model, compare normalized live and desired schemas, replay migrations in an isolated database, and apply migrations using the configured runner. Database-backed commands SHALL report SQLite as unsupported until each corresponding capability is wired and SHALL advertise only working capabilities.

#### Scenario: Inspect a SQLite database
- **WHEN** the inspect command receives a valid SQLite database location
- **THEN** it SHALL emit a deterministic snapshot-compatible representation of supported tables, columns, constraints, indexes, views, and triggers

#### Scenario: Normalize SQLite catalog details before comparison
- **WHEN** declared and introspected schemas are compared
- **THEN** normalization SHALL account for SQLite type affinity, rowid-backed primary keys, auto-generated index names, generated expressions, and SQLite catalog formatting without hiding meaningful differences

#### Scenario: Report object-level drift
- **WHEN** the live database differs from the desired snapshot
- **THEN** drift check SHALL identify missing, extra, and changed supported objects and return a non-success result

#### Scenario: Replay and apply track migration state
- **WHEN** SQLite migrations are replayed or applied
- **THEN** tooling SHALL enforce the configured foreign-key posture and report the runner's migration execution state

#### Scenario: Replay migrations in an isolated SQLite database
- **WHEN** sandbox replay is requested
- **THEN** migrations SHALL run against a disposable SQLite database and the resulting introspected schema SHALL match the migration's embedded target snapshot

#### Scenario: Apply and track SQLite migrations
- **WHEN** a migration is applied with goose or golang-migrate selected
- **THEN** tooling SHALL execute the configured migration format, maintain that runner's version state, and skip already-applied versions

#### Scenario: Enforce foreign-key connection posture
- **WHEN** an SQLite tooling connection is opened for inspection, replay, or apply
- **THEN** its configured foreign-key enforcement SHALL be enabled and restored or reported consistently across connection reuse

#### Scenario: Reject unavailable SQLite tooling capabilities
- **WHEN** a SQLite command requests inspection, drift, sandbox replay, or apply before that behavior is available
- **THEN** the command SHALL return a clear unsupported-capability error rather than entering PostgreSQL tooling

### Requirement: SQLite migration plans can use a live database as their source
SQLite migration planning SHALL introspect a configured source database when the user requests a live-source plan, then compare that SQLite snapshot with the desired SQLite schema through the SQLite planner.

#### Scenario: Plan from a live SQLite source
- **WHEN** `migrate plan` or `migrate create` is configured with a live SQLite source location
- **THEN** the source SHALL be introspected as SQLite and the resulting plan SHALL use SQLite migration semantics without routing through PostgreSQL tooling

#### Scenario: Reject an unrepresentable live source
- **WHEN** source introspection encounters a SQLite schema form that cannot be represented safely
- **THEN** planning SHALL return an actionable error and SHALL NOT write a partial migration

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
