# Handover: Move PG-only index options out of the shared AST

## Problem

`internal/ast/schema.go` is the dialect-neutral schema core, but it leaks
PostgreSQL-specific fields into shared types:

```go
// internal/ast/schema.go — current
type Index struct {
    With         map[string]string  // PG-only (storage params)
    Method       string             // PG-only (btree, hash, gist, ...)
    Concurrently bool               // PG-only (CREATE INDEX CONCURRENTLY)
    Only         bool               // PG-only (ONLY for partitioned tables)
    // shared: Name, PreviousName, Columns, Where, Unique
}

type IndexColumn struct {
    OpClass string  // PG-only (operator class)
    Nulls   string  // PG-only (NULLS FIRST/LAST)
    // shared: Expression, Order, IsExpression
}

type ExclusionConstraint struct {
    Method   string              // PG-only (EXCLUDE is a PG concept entirely)
    Elements []ExclusionElement
    // shared: Name, PreviousName, Where, Initially, Deferrable
}

type ExclusionElement struct {
    OpClass string  // PG-only
    Nulls   string  // PG-only
    // shared: Expression, Operator, Order
}
```

Nothing from `internal/dialects/pg/` should escape into `internal/ast/`.

## Constraints

1. **`ast` stays dialect-neutral** — no PG-specific fields on any `ast` type.
2. **Strong typing** — no `any`, no `[]interface{}`, no type assertions at
   call sites.
3. **JSON snapshot must round-trip** — `json.Marshal` and `json.Unmarshal`
   must both work on the snapshot document. (The migration planner
   unmarshals snapshots into `pgschema.Document`.)
4. **JSON shape should not change** — the snapshot format is versioned and
   consumed by the planner; a nested/wrapped JSON structure would be a
   breaking change.
5. **The DSL `Element.apply` pattern must keep working** — table elements
   (columns, indexes, constraints) are built via an `apply(table)` interface.
6. **Extensible** — a second dialect (MySQL, etc.) should be able to add its
   own index options without touching `ast`.

## Proposed design: embed + shadow

The key insight: Go struct embedding with **field shadowing** produces flat
JSON and gives strong typing, with no interfaces or `any`.

### Step 1: Strip `ast` types to shared fields only

```go
// internal/ast/schema.go

type Index struct {
    Name         string        `json:"name"`
    PreviousName string        `json:"previousName,omitempty"`
    Columns      []IndexColumn `json:"columns,omitempty"`
    Where        string        `json:"where,omitempty"`
    Unique       bool          `json:"unique,omitempty"`
}

type IndexColumn struct {
    Expression   string `json:"expression"`
    Order        string `json:"order,omitempty"`
    IsExpression bool   `json:"isExpression,omitempty"`
}

type ExclusionConstraint struct {
    Name         string             `json:"name"`
    PreviousName string             `json:"previousName,omitempty"`
    Where        string             `json:"where,omitempty"`
    Initially    string             `json:"initially,omitempty"`
    Elements     []ExclusionElement `json:"elements,omitempty"`
    Deferrable   bool               `json:"deferrable,omitempty"`
}

type ExclusionElement struct {
    Expression string `json:"expression"`
    Operator   string `json:"operator"`
    Order      string `json:"order,omitempty"`
}
```

`Method`, `Concurrently`, `Only`, `With`, `OpClass`, `Nulls` are all gone.

### Step 2: Create PG-specific types via embed + shadow

```go
// internal/dialects/pg/pgschema/index.go

type Index struct {
    ast.Index                          // embeds: Name, PreviousName, Where, Unique
    Columns      []IndexColumn         `json:"columns,omitempty"`         // shadows ast.Index.Columns
    Method       string                `json:"method,omitempty"`
    Concurrently bool                  `json:"concurrently,omitempty"`
    Only         bool                  `json:"only,omitempty"`
    With         map[string]string     `json:"with,omitempty"`
}

type IndexColumn struct {
    ast.IndexColumn                    // embeds: Expression, Order, IsExpression
    OpClass string `json:"opClass,omitempty"`
    Nulls   string `json:"nulls,omitempty"`
}
```

```go
// internal/dialects/pg/pgschema/exclusion.go

type ExclusionConstraint struct {
    ast.ExclusionConstraint            // embeds: Name, PreviousName, Where, Initially, Deferrable
    Method   string             `json:"method,omitempty"`
    Elements []ExclusionElement `json:"elements,omitempty"`   // shadows ast.ExclusionConstraint.Elements
}

type ExclusionElement struct {
    ast.ExclusionElement               // embeds: Expression, Operator, Order
    OpClass string `json:"opClass,omitempty"`
    Nulls   string `json:"nulls,omitempty"`
}
```

### Step 3: `pgschema.Table` embeds `ast.Table`, shadows `Indexes` + `Exclusions`

```go
// internal/dialects/pg/pgschema/table.go

type Table struct {
    ast.Table                                      // embeds: Columns, PrimaryKeys, FKs, Checks, Schema, Name, etc.
    Indexes    []Index             `json:"indexes,omitempty"`       // shadows ast.Table.Indexes
    Exclusions []ExclusionConstraint `json:"exclusions,omitempty"`   // shadows ast.Table.Exclusions
}
```

### Why this works

**JSON marshaling is flat:**
Go's `encoding/json` marshals embedded struct fields at the top level, but
when the outer struct has a field with the same JSON tag as an embedded
field, the **outer field wins** (the embedded one is omitted). So:

```json
{
  "name": "my_index",
  "columns": [{"expression": "email", "opClass": "text_ops"}],
  "method": "btree",
  "concurrently": true,
  "unique": false
}
```

— flat, same shape as today, all PG fields included.

**JSON unmarshaling works:**
Same rule in reverse — the outer field is populated. The planner can
unmarshal snapshots into `pgschema.Document` (which now has
`Tables []pgschema.Table`) and get fully typed `pgschema.Index` objects.

**Strong typing throughout:**
- `pgschema.Table.Indexes` is `[]pgschema.Index` (not `[]any`).
- `pgschema.Index.Columns` is `[]pgschema.IndexColumn`.
- The renderer, planner, and introspector all use concrete `pgschema.*`
  types. No type assertions.

**Shared fields via embedding:**
`pgschema.Table.Columns`, `.PrimaryKeys`, `.ForeignKeys`, `.Checks`, etc.
all come from `ast.Table` via embedding promotion. No duplication.

**Extensible for other dialects:**
A future `mysqlschema.Table` would embed `ast.Table` and shadow `Indexes`
with `[]mysqlschema.Index`. The shared `ast` package is never touched.

### Step 4: Update the pipeline

| Layer | Current | After |
|-------|---------|-------|
| `ast.Table` | Full table struct with `[]ast.Index` | Shared base only (no PG index fields) |
| `pgschema.Table` | Does not exist | Embeds `ast.Table`, shadows `Indexes`/`Exclusions` with PG types |
| `pgschema.Schema.Tables` | `[]ast.Table` | `[]pgschema.Table` |
| `pgschema.Document.Tables` | `[]ast.Table` | `[]pgschema.Table` |
| `pg.IndexDef.def` | `ast.Index` | `pgschema.Index` |
| `pg.ExclusionDef.def` | `ast.ExclusionConstraint` | `pgschema.ExclusionConstraint` |
| `pg.Element.apply` | `apply(table *ast.Table)` | `apply(table *pgschema.Table)` |
| `pg.Definition.def` | `*ast.Table` | `*pgschema.Table` |
| `pg.registry.tables` | `[]*ast.Table` | `[]*pgschema.Table` |
| `render.Postgres` | takes `pgschema.Schema` with `[]ast.Table` | same, but `Tables` is `[]pgschema.Table` |
| `render.renderTable` | `renderTable(b, table ast.Table)` | `renderTable(b, table pgschema.Table)` |
| `render.renderIndex` | `renderIndex(b, name, index ast.Index)` | `renderIndex(b, name, index pgschema.Index)` |
| `render.validateIndex` | `validateIndex(name, index ast.Index, cols)` | `validateIndex(name, index pgschema.Index, cols)` |
| `plan.SnapshotDiff` | unmarshals into `pgschema.Document` with `[]ast.Table` | same, but `Tables` is `[]pgschema.Table` |
| `plan.tables.go` | diffs `ast.Table.Indexes` | diffs `pgschema.Table.Indexes` |
| `plan.render.go` | `renderIndex(table ast.Table, index ast.Index)` | `renderIndex(table pgschema.Table, index pgschema.Index)` |
| `tooling/introspect.go` | builds `ast.Table`, `ast.IndexColumn` | builds `pgschema.Table`, `pgschema.IndexColumn` |
| `postgres_test.go` | builds `pgschema.Schema` with `[]ast.Table` | builds with `[]pgschema.Table` |

### What does NOT change

- `ast.Column`, `ast.PrimaryKey`, `ast.UniqueConstraint`,
  `ast.ForeignKeyConstraint`, `ast.Check` — these are genuinely shared and
  stay in `ast`.
- The JSON snapshot shape — same field names, same flat structure.
- The DSL public API (`pg.Table()`, `pg.Index()`, `pg.IndexOn()`, etc.) —
  same function signatures, same builder methods.
- The `kit.Provider` interface.
- The CLI layer.

## Alternatives considered and rejected

### A. Interface on `ast.Index`

```go
type Index interface { Name() string; Columns() []IndexColumn; ... }
```

**Rejected:** `encoding/json` cannot unmarshal into an interface value — it
produces `map[string]interface{}`. The planner unmarshals snapshots, so this
breaks round-tripping. Would require custom `UnmarshalJSON` on every
consumer.

### B. `ast.Table.Indexes` as `[]any`

**Rejected:** Loses strong typing. Every access site needs a type assertion.
JSON unmarshaling produces `[]interface{}` of maps, not typed structs.
Requires post-processing after every unmarshal.

### C. Generic `Table[T Index]`

**Rejected:** Generics on the table struct would force the renderer, planner,
and introspector to all be generic functions. Extremely high complexity for
no real benefit over embed+shadow.

### D. Separate `IndexMetadata` map on `pgschema.Schema`

Store PG index options in a side-map keyed by `table.indexName`, keep
`ast.Index` minimal.

**Rejected:** Splits index data across two locations. The migration planner
would need to diff both `tables[].indexes[]` and the metadata map.
Fragile and hard to keep in sync.

### E. Embed + shadow (this proposal)

**Accepted:** Flat JSON, strong typing, no interfaces, no `any`, no custom
marshalers. Uses a standard Go language feature. Extensible to other
dialects. The only cost is changing `pgschema.Schema.Tables` from
`[]ast.Table` to `[]pgschema.Table`, which cascades through the renderer,
planner, and introspector — but those are all in `internal/dialects/pg/`
already.

## Impact summary

Files that change (all under `internal/dialects/pg/` or `pg/`):

- `internal/ast/schema.go` — strip PG fields (additive: also add `Collation`
  to `Column`, `Tablespace`/`PartitionBy` to `Table` for other features)
- `internal/dialects/pg/pgschema/index.go` — NEW: `Index`, `IndexColumn`
- `internal/dialects/pg/pgschema/exclusion.go` — NEW: `ExclusionConstraint`, `ExclusionElement`
- `internal/dialects/pg/pgschema/table.go` — NEW: `Table` type
- `internal/dialects/pg/pgschema/schema.go` — `Schema.Tables` and
  `Document.Tables` become `[]pgschema.Table`
- `pg/table.go` — `Element.apply` takes `*pgschema.Table`; `IndexDef`,
  `ExclusionDef` use `pgschema.*` types
- `pg/registry.go` — stores `[]*pgschema.Table`
- `internal/dialects/pg/render/postgres.go` — all `ast.Table`/`ast.Index`
  params become `pgschema.*`
- `internal/dialects/pg/plan/tables.go` — diff functions use `pgschema.*`
- `internal/dialects/pg/plan/render.go` — `renderIndex` uses `pgschema.Index`
- `internal/dialects/pg/plan/keys.go` — `tableKey` takes `pgschema.Table`
- `internal/dialects/pg/tooling/introspect.go` — builds `pgschema.*` types
- `internal/dialects/pg/render/postgres_test.go` — test schema uses
  `pgschema.*` types
- Golden files — regenerate (`task generate && task snapshot`)

Files that do NOT change:
- `kit/registry.go`
- `internal/cli/*`
- `internal/app/*`
- `cmd/gosqlkit/main.go`
- `examples/basic/` (DSL public API is unchanged)
