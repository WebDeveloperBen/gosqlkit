# Spec Delta

## MODIFIED Requirements

### Requirement: SQLite database tooling supports inspection, drift, replay, and apply
SQLite tooling SHALL connect to a SQLite file or in-memory database, introspect supported schema objects into the SQLite snapshot model, compare normalized live and desired schemas, replay migrations in an isolated database, and apply migrations using the configured runner. Database-backed commands SHALL report SQLite as unsupported until each corresponding capability is wired and SHALL advertise only working capabilities.

#### Scenario: Inspect a SQLite database
- **WHEN** the inspect command receives a valid SQLite database location
- **THEN** it SHALL emit a deterministic snapshot-compatible representation of supported tables, columns, constraints, indexes, views, and triggers

#### Scenario: Normalize SQLite catalog details before comparison
- **WHEN** declared and introspected schemas are compared
- **THEN** normalization SHALL account for SQLite type affinity, rowid-backed primary keys, auto-generated index names, generated expressions, and SQLite catalog formatting without hiding meaningful differences

#### Scenario: Report object-level drift
- **WHEN** the live database differs from the desired snapshot
- **THEN** drift check SHALL identify missing, extra, and changed supported objects and return a non-success result

#### Scenario: Compare drift using SQLite semantics
- **WHEN** desired and introspected SQLite schemas are compared
- **THEN** comparison SHALL account for SQLite type affinity, rowid primary keys, generated expressions, and catalog normalization

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

## ADDED Requirements

### Requirement: SQLite migration plans can use a live database as their source
SQLite migration planning SHALL introspect a configured source database when the user requests a live-source plan, then compare that SQLite snapshot with the desired SQLite schema through the SQLite planner.

#### Scenario: Plan from a live SQLite source
- **WHEN** `migrate plan` or `migrate create` is configured with a live SQLite source location
- **THEN** the source SHALL be introspected as SQLite and the resulting plan SHALL use SQLite migration semantics without routing through PostgreSQL tooling

#### Scenario: Reject an unrepresentable live source
- **WHEN** source introspection encounters a SQLite schema form that cannot be represented safely
- **THEN** planning SHALL return an actionable error and SHALL NOT write a partial migration
