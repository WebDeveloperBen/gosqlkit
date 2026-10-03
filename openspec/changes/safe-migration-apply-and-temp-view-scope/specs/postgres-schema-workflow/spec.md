# Spec Delta

## ADDED Requirements

### Requirement: PostgreSQL transactional migration application is atomic
PostgreSQL migration application SHALL execute transaction-compatible migration statements and successful runner state atomically; operations that cannot run in a transaction SHALL retain explicit non-transactional runner semantics and report recoverable failure state.

#### Scenario: Transactional migration fails after applying statements
- **WHEN** a PostgreSQL migration fails after one or more transaction-compatible statements have executed
- **THEN** those migration changes SHALL roll back, no successful runner version SHALL be recorded, and the error SHALL identify the failed migration

#### Scenario: Transaction-compatible migration succeeds
- **WHEN** all statements in a PostgreSQL migration are transaction-compatible and execute successfully
- **THEN** the migration changes and successful runner state SHALL commit together

#### Scenario: Migration contains a non-transactional operation
- **WHEN** a supported PostgreSQL migration operation cannot run inside a transaction
- **THEN** the selected runner's explicit non-transactional behavior SHALL be preserved, failures SHALL not be reported as atomic rollbacks, and runner state SHALL indicate that recovery is required
