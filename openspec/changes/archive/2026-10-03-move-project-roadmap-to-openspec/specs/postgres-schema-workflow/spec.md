# Spec Delta

## Purpose

Defines the supported PostgreSQL schema-as-code workflow, from Go declarations and deterministic snapshots through reviewable migration planning and database tooling.

## ADDED Requirements

### Requirement: PostgreSQL declarations model supported native schema objects
The PostgreSQL DSL SHALL model tables, columns, indexes, constraints, schemas, extensions, enums, composite types, domains, sequences, collations, roles, functions, views, materialized views, triggers, policies, grants, and ordered raw SQL blocks.

#### Scenario: Declare PostgreSQL column semantics
- **WHEN** a schema defines supported scalar, array, custom, identity, generated, or defaulted columns
- **THEN** the model and SQL SHALL preserve those declared type and column options

#### Scenario: Declare PostgreSQL constraints and indexes
- **WHEN** a schema defines supported primary, unique, foreign-key, check, or exclusion constraints and index options
- **THEN** the SQL SHALL preserve constraint actions, deferrability, index method, opclass, ordering, null ordering, expressions, predicates, concurrency, ONLY, and storage parameters

#### Scenario: Declare PostgreSQL schema-level objects
- **WHEN** a schema defines supported PostgreSQL objects or raw SQL blocks
- **THEN** the renderer SHALL emit their canonical statements in dependency-safe order

### Requirement: PostgreSQL rendering is deterministic and validates schema definitions
The PostgreSQL renderer SHALL produce stable canonical SQL and SHALL reject invalid identifiers, references, conflicts, and dialect-specific option combinations with an error.

#### Scenario: Invalid schema cannot produce SQL
- **WHEN** a schema contains an invalid or conflicting declaration
- **THEN** rendering and snapshot generation SHALL return a contextual validation error

#### Scenario: Equivalent declarations render consistently
- **WHEN** equivalent schemas are supplied in different collection orders
- **THEN** canonical SQL ordering SHALL remain stable

### Requirement: PostgreSQL snapshots are deterministic and diff-ready
The PostgreSQL provider SHALL emit versioned, dialect-tagged snapshot JSON with stable snapshot identifiers, object metadata, rename annotations, and deterministic object ordering.

#### Scenario: Equivalent schema produces stable snapshot
- **WHEN** the same schema is collected repeatedly
- **THEN** its normalized snapshot content and identifier SHALL be stable

#### Scenario: Snapshot preserves modeled object and metadata fields
- **WHEN** a supported object is included in a schema
- **THEN** its modeled fields and metadata SHALL be serialized in the snapshot

### Requirement: PostgreSQL migration plans preserve review and safety information
Migration planning SHALL represent supported changes with object identity, dependencies, reversibility, and risk; destructive or unsupported changes SHALL fail closed unless an explicit supported override applies.

#### Scenario: Destructive change is guarded
- **WHEN** a plan contains a destructive change and destructive authoring is not enabled
- **THEN** migration creation SHALL refuse to write the migration and explain the required action

#### Scenario: Unsupported change cannot emit partial migration SQL
- **WHEN** a schema difference has no supported executable plan or combines operations that cannot be safely composed
- **THEN** planning SHALL return an actionable error without writing partial SQL

#### Scenario: Reversible operations include reverse SQL where supported
- **WHEN** a planned operation has a well-defined safe reversal
- **THEN** the migration plan SHALL include reverse SQL for that operation

### Requirement: PostgreSQL database tooling compares database state with desired snapshots
PostgreSQL tooling SHALL introspect supported catalog objects into a snapshot-compatible model and SHALL support database inspection, drift checks, sandbox migration replay, and migration application for the configured runner.

#### Scenario: Drift check reports schema differences
- **WHEN** a live PostgreSQL database differs from the desired schema snapshot
- **THEN** the drift command SHALL report missing, extra, and changed supported objects

#### Scenario: Sandbox replay verifies migration target
- **WHEN** committed migrations are replayed in a PostgreSQL sandbox
- **THEN** the replay check SHALL compare introspected state with the embedded target snapshot

#### Scenario: Apply tracks runner migration state
- **WHEN** a migration is applied successfully
- **THEN** the selected runner's database version state SHALL record it so subsequent apply skips it

### Requirement: PostgreSQL database authentication supports configured token providers
Database-backed PostgreSQL commands SHALL support configured URL, environment, command, and supported cloud token authentication without exposing token values in normal command output.

#### Scenario: Connect using a configured token provider
- **WHEN** a database-backed command is configured with a supported token source
- **THEN** the connection SHALL use the resolved token as credentials without persisting it to the schema snapshot
