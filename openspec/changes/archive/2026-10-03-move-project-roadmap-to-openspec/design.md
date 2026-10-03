# Design

## Context

See `proposal.md` for motivation and `specs/` for the capability contracts. The repository already has `openspec/config.yaml` using `spec-driven`, but no published capability specs or active changes. `FEATURES.md` and `SQLITE.md` contain large checklists; `SQLITE.md` inconsistently marks planner and CLI capabilities as missing despite code, tests, examples, and its own handoff describing them as landed. `AGENTS.md` repeats a point-in-time summary. Six OpenSpec commands and six skills currently live under `.claude`; `.agents/` does not exist.

## Goals / Non-Goals

**Goals:**
- Make OpenSpec capability specs the durable behavior contract and change task lists the source for unfinished implementation.
- Record verified PostgreSQL and SQLite behavior without treating stale checklist marks as evidence.
- Give SQLite's remaining tooling work a dedicated, actionable change task list.
- Keep useful rationale, architecture, and user instructions in existing documents while removing competing status boards.

**Non-Goals:**
- Change application code, SQLite support, or CLI behavior as part of this documentation migration.
- Declare every SQLite requirement implemented merely because a roadmap section says it is done.
- Delete `SPEC.md`, `MIGRATIONS.md`, or all of `AGENTS.md` / `README.md`.

## Decisions

1. **Separate behavior contracts from implementation status.** `openspec/specs/` records what each dialect is required to do, including SQLite requirements not yet implemented. The focused SQLite tooling change records the remaining work as unchecked tasks. During migration, mark migration tasks complete only when current source/tests/examples support the claim. This avoids falsely using normative specs as completion checklists.

2. **Use three cohesive capabilities.** Keep PostgreSQL and SQLite schema workflows in separate specs because their object models and SQL semantics differ. Put the repository's status-source convention in a small process capability rather than mixing it into the database behavior specs.

3. **Audit from code outward.** Use `FEATURES.md` and `SQLITE.md` as discovery indexes, then verify each requirement against the public DSL, renderer, planner, app/CLI routing, tests, examples, and Taskfile. Preserve explicit SQLite non-equivalents as unsupported constraints, not unfinished tasks. Record discrepancies rather than silently resolving ambiguous product claims.

4. **Treat migration commands as independently supported surfaces.** The SQLite example's `generate --check` and `snapshot --check` pass. `migrate plan --json` requires a prior migration baseline; the example currently has none. Record this prerequisite instead of labeling the command broken or claiming it was smoke-verified.

5. **Use the shared `.agents/` location for OpenSpec assets.** Preserve the existing relative layout under `.agents/commands/opsx/` and `.agents/skills/openspec-*/`, move all six commands and six skills, and remove those OpenSpec copies from `.claude/`. Keep unrelated `.claude/settings.local.json` untouched. `.agents/skills/` is the shared skill location; verify command discovery rather than assuming `.agents/commands/` is automatically invoked by every agent. Tailor `.agents/skills/openspec-propose/SKILL.md` with repo-specific evidence and status rules.

## Risks / Trade-offs

- [The current roadmap contains stale and internally conflicting claims] -> Treat source, tests, runnable examples, and current capability flags as evidence; flag anything not verified instead of assuming completion.
- [Broad feature checklists can become unwieldy as specs] -> Group requirements by externally observable behavior and dialect; keep unfinished implementation steps in change tasks, not as nested status lists in specs.
- [Replacing docs can lose rationale or user guidance] -> Preserve product decisions and user instructions; only remove duplicated status tracking and replace it with links.
- [A full audit can miss obscure behavior] -> Include explicit task-level verification for each capability area and leave unverified details as open work rather than claiming parity.

## Migration Plan

1. Publish the three capability specs and validate their requirement/scenario structure.
2. Audit the implementation and mark the roadmap-migration tasks complete only with evidence; reconcile stale checklist entries.
3. Create a separate SQLite tooling change from verified outstanding requirements, with actionable tasks for inspection/introspection, drift normalization, sandbox replay, apply/runner support, and embedded integration coverage.
4. Move the six OpenSpec command files and six skill directories into the matching `.agents/` layout, remove their old `.claude` copies, and verify files plus agent discovery.
5. Update repository proposal guidance and status references; retain architectural and decision documents.
6. Run OpenSpec validation and targeted read-only example checks. No database behavior changes, code tests, or deployment rollback are involved.

## Open Questions

None that block this plan. The detailed treatment of each checklist item is decided during the evidence audit; an item without implementation or test evidence remains incomplete or is recorded as a deliberate SQLite non-equivalent.