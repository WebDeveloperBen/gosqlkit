# Spec Delta

## Purpose

Defines how repository capability contracts and unfinished implementation work are recorded so maintainers can identify verified behavior and resume work without conflicting status lists.

## ADDED Requirements

### Requirement: OpenSpec is the canonical capability and work status source
Durable product behavior SHALL be specified in OpenSpec capability specs, and in-flight implementation SHALL be tracked by change tasks; repository documents SHALL NOT maintain competing completion checklists.

#### Scenario: Maintainer identifies supported behavior
- **WHEN** a maintainer needs the expected behavior of a capability
- **THEN** the corresponding OpenSpec capability spec SHALL be the canonical contract

#### Scenario: Maintainer identifies unfinished work
- **WHEN** implementation work remains
- **THEN** an active OpenSpec change SHALL provide ordered, checkable tasks for that work

### Requirement: OpenSpec proposals are grounded in current repository evidence
Repository proposal guidance SHALL require checking existing OpenSpec specs, implementation, tests, examples, and relevant documentation before defining scope or claiming work is complete.

#### Scenario: Propose work in an area with existing implementation
- **WHEN** a proposal concerns an existing capability
- **THEN** the proposer SHALL distinguish verified existing behavior from proposed changes and SHALL avoid duplicating existing requirements

#### Scenario: Track already implemented behavior
- **WHEN** a capability spec is created from existing project documentation
- **THEN** claims marked complete SHALL be supported by current code, tests, or runnable examples
