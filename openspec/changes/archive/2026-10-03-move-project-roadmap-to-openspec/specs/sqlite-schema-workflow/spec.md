# Spec Delta

## Purpose

Defines SQLite's supported schema-as-code, snapshot, and migration behavior, preserving SQLite-specific semantics and separating implemented workflow from remaining database tooling.

## ADDED Requirements

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
SQLite tooling SHALL connect to a SQLite file or in-memory database, introspect modeled objects, normalize SQLite catalog representations, and support drift checking, sandbox replay, and migration application.

#### Scenario: Inspect a SQLite database
- **WHEN** the inspect command receives a SQLite database location
- **THEN** it SHALL emit a deterministic snapshot-compatible representation of supported objects

#### Scenario: Compare drift using SQLite semantics
- **WHEN** desired and introspected SQLite schemas are compared
- **THEN** comparison SHALL account for SQLite type affinity, rowid primary keys, generated expressions, and catalog normalization

#### Scenario: Replay and apply track migration state
- **WHEN** SQLite migrations are replayed or applied
- **THEN** tooling SHALL enforce the configured foreign-key posture and report the runner's migration execution state
