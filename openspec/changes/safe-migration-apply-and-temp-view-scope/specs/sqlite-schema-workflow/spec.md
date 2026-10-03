# Spec Delta

## ADDED Requirements

### Requirement: SQLite transactional migration application is atomic
SQLite migration application SHALL commit transaction-compatible migration statements and successful runner state atomically, including table rebuilds, and SHALL restore the configured foreign-key posture after either success or failure.

#### Scenario: Rebuild failure rolls back schema changes
- **WHEN** a SQLite rebuild migration fails after one or more rebuild statements have executed
- **THEN** the migration's schema and data changes SHALL be rolled back, no successful migration state SHALL be recorded, and foreign-key enforcement SHALL be restored and verified

#### Scenario: Successful rebuild commits schema and runner state together
- **WHEN** a SQLite rebuild migration completes successfully
- **THEN** its rebuilt schema and successful runner state SHALL commit together, and foreign-key enforcement SHALL be enabled afterward

### Requirement: Temporary views are excluded from live database comparison
SQLite temporary views SHALL remain in declared SQL and schema snapshots but SHALL be excluded from live database inspection and drift comparison because they are connection-local.

#### Scenario: Compare a schema containing a temporary view
- **WHEN** live inspection or drift checking runs in a new tooling connection for a schema that declares a temporary view
- **THEN** the declaration snapshot SHALL preserve the temporary view, while live database output and drift results SHALL not report it as a missing or extra persistent object
