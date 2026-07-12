package sqlite

import (
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

// ViewDef builds a SQLite view. It is registered on construction; builder
// methods mutate the registered value through a pointer.
type ViewDef struct {
	def *sqliteschema.View
}

func View(name string, query ...string) *ViewDef {
	view := &sqliteschema.View{Name: name}
	if len(query) > 0 {
		view.Query = strings.Join(query, "\n")
	}
	registerView(view)
	return &ViewDef{def: view}
}

func (v *ViewDef) As(query string) *ViewDef {
	v.def.Query = query
	return v
}

func (v *ViewDef) Columns(columns ...string) *ViewDef {
	v.def.ColumnAliases = columns
	return v
}

func (v *ViewDef) Temporary() *ViewDef {
	v.def.Temporary = true
	return v
}

func (v *ViewDef) DependsOn(dependencies ...string) *ViewDef {
	v.def.DependsOn = append(v.def.DependsOn, dependencies...)
	return v
}

func (v *ViewDef) Comment(text string) *ViewDef {
	v.def.Comment = text
	return v
}

// TriggerDef builds a SQLite trigger.
type TriggerDef struct {
	def *sqliteschema.Trigger
}

func Trigger(name, target string) *TriggerDef {
	trigger := &sqliteschema.Trigger{Name: name, Target: target}
	registerTrigger(trigger)
	return &TriggerDef{def: trigger}
}

func (t *TriggerDef) Before() *TriggerDef {
	t.def.Timing = "before"
	return t
}

func (t *TriggerDef) After() *TriggerDef {
	t.def.Timing = "after"
	return t
}

func (t *TriggerDef) InsteadOf() *TriggerDef {
	t.def.Timing = "instead of"
	return t
}

func (t *TriggerDef) Insert() *TriggerDef {
	t.def.Events = []string{"insert"}
	return t
}

func (t *TriggerDef) Update() *TriggerDef {
	t.def.Events = []string{"update"}
	return t
}

func (t *TriggerDef) UpdateOf(columns ...string) *TriggerDef {
	t.def.Events = []string{"update"}
	t.def.UpdateOfColumns = columns
	return t
}

func (t *TriggerDef) Delete() *TriggerDef {
	t.def.Events = []string{"delete"}
	return t
}

func (t *TriggerDef) ForEachRow() *TriggerDef {
	t.def.ForEachRow = true
	return t
}

func (t *TriggerDef) When(condition string) *TriggerDef {
	t.def.When = condition
	return t
}

// Body sets the trigger body statements (rendered between BEGIN and END).
func (t *TriggerDef) Body(statements ...string) *TriggerDef {
	t.def.Body = strings.Join(statements, "\n")
	return t
}

func (t *TriggerDef) Comment(text string) *TriggerDef {
	t.def.Comment = text
	return t
}

// RawSQLDef is an escape hatch for DDL the structured DSL does not model.
type RawSQLDef struct {
	def *sqliteschema.RawSQL
}

func RawSQL(name, sql string) *RawSQLDef {
	block := &sqliteschema.RawSQL{Name: name, SQL: sql}
	registerRawSQL(block)
	return &RawSQLDef{def: block}
}

// Before renders the block before the structured schema (default is after).
func (r *RawSQLDef) Before() *RawSQLDef {
	r.def.Before = true
	return r
}

// Down sets the reverse SQL used when the block participates in down migrations.
func (r *RawSQLDef) Down(sql string) *RawSQLDef {
	r.def.Down = sql
	return r
}
