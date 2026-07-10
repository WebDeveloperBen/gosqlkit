package pg

import (
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

type NamespaceDef struct {
	def pgschema.Namespace
}

type ExtensionDef struct {
	def *pgschema.Extension
}

type RoleDef struct {
	def *pgschema.Role
}

type CollationDef struct {
	def *pgschema.Collation
}

type EnumDef struct {
	def pgschema.Enum
}

type SequenceDef struct {
	def pgschema.Sequence
}

type CompositeTypeDef struct {
	def pgschema.CompositeType
}

type CompositeAttributeDef struct {
	def pgschema.CompositeAttribute
}

type DomainDef struct {
	def *pgschema.Domain
}

type FunctionDef struct {
	def *pgschema.Function
}

type FunctionArgumentDef struct {
	def pgschema.FunctionArgument
}

type TriggerDef struct {
	def *pgschema.Trigger
}

type PolicyDef struct {
	def *pgschema.Policy
}

type GrantDef struct {
	def *pgschema.Grant
}

type Privilege string

type CollationProvider string

const (
	PrivilegeAll        Privilege = "ALL PRIVILEGES"
	PrivilegeSelect     Privilege = "SELECT"
	PrivilegeInsert     Privilege = "INSERT"
	PrivilegeUpdate     Privilege = "UPDATE"
	PrivilegeDelete     Privilege = "DELETE"
	PrivilegeTruncate   Privilege = "TRUNCATE"
	PrivilegeReferences Privilege = "REFERENCES"
	PrivilegeTrigger    Privilege = "TRIGGER"
	PrivilegeMaintain   Privilege = "MAINTAIN"
	PrivilegeUsage      Privilege = "USAGE"
	PrivilegeExecute    Privilege = "EXECUTE"
	PrivilegeConnect    Privilege = "CONNECT"
	PrivilegeCreate     Privilege = "CREATE"
	PrivilegeTemporary  Privilege = "TEMPORARY"
)

const (
	CollationProviderBuiltin CollationProvider = "builtin"
	CollationProviderICU     CollationProvider = "icu"
	CollationProviderLibc    CollationProvider = "libc"
)

func Namespace(name string) *NamespaceDef {
	namespace := pgschema.Namespace{Name: name}
	registerNamespace(namespace)
	return &NamespaceDef{def: namespace}
}

type RawSQLDef struct {
	def *pgschema.RawSQL
}

// RawSQL registers an arbitrary SQL DDL block as an escape hatch for schema
// objects the structured DSL does not model. Blocks render after the
// structured schema by default; call Before to render ahead of it.
func RawSQL(name, sql string) *RawSQLDef {
	block := &pgschema.RawSQL{Name: name, SQL: sql}
	registerRawSQL(block)
	return &RawSQLDef{def: block}
}

func (r *RawSQLDef) Before() *RawSQLDef {
	r.def.Before = true
	return r
}

// Down supplies the reverse SQL for this block so it can participate in
// down migrations. Without it the block is marked with an irreversible
// placeholder in generated down SQL.
func (r *RawSQLDef) Down(sql string) *RawSQLDef {
	r.def.Down = sql
	return r
}

func Extension(name string) *ExtensionDef {
	return ExtensionInSchema("", name)
}

func ExtensionInSchema(schema, name string) *ExtensionDef {
	extension := &pgschema.Extension{Schema: schema, Name: name}
	registerExtension(extension)
	return &ExtensionDef{def: extension}
}

func (e *ExtensionDef) Version(version string) *ExtensionDef {
	e.def.Version = version
	return e
}

func (e *ExtensionDef) Cascade() *ExtensionDef {
	e.def.Cascade = true
	return e
}

func Role(name string) *RoleDef {
	role := &pgschema.Role{Name: name}
	registerRole(role)
	return &RoleDef{def: role}
}

func (r *RoleDef) Login() *RoleDef {
	r.def.Login = new(true)
	return r
}

func (r *RoleDef) NoLogin() *RoleDef {
	r.def.Login = new(false)
	return r
}

func (r *RoleDef) Superuser() *RoleDef {
	r.def.Superuser = new(true)
	return r
}

func (r *RoleDef) NoSuperuser() *RoleDef {
	r.def.Superuser = new(false)
	return r
}

func (r *RoleDef) CreateDB() *RoleDef {
	r.def.CreateDB = new(true)
	return r
}

func (r *RoleDef) NoCreateDB() *RoleDef {
	r.def.CreateDB = new(false)
	return r
}

func (r *RoleDef) CreateRole() *RoleDef {
	r.def.CreateRole = new(true)
	return r
}

func (r *RoleDef) NoCreateRole() *RoleDef {
	r.def.CreateRole = new(false)
	return r
}

func (r *RoleDef) Inherit() *RoleDef {
	r.def.Inherit = new(true)
	return r
}

func (r *RoleDef) NoInherit() *RoleDef {
	r.def.Inherit = new(false)
	return r
}

func (r *RoleDef) Replication() *RoleDef {
	r.def.Replication = new(true)
	return r
}

func (r *RoleDef) NoReplication() *RoleDef {
	r.def.Replication = new(false)
	return r
}

func (r *RoleDef) BypassRLS() *RoleDef {
	r.def.BypassRLS = new(true)
	return r
}

func (r *RoleDef) NoBypassRLS() *RoleDef {
	r.def.BypassRLS = new(false)
	return r
}

func (r *RoleDef) ConnectionLimit(limit int) *RoleDef {
	r.def.ConnectionLimit = &limit
	return r
}

func (r *RoleDef) ValidUntil(value string) *RoleDef {
	r.def.ValidUntil = value
	return r
}

func (r *RoleDef) MemberOf(roles ...string) *RoleDef {
	r.def.MemberOf = append(r.def.MemberOf, roles...)
	return r
}

func (r *RoleDef) AdminOf(roles ...string) *RoleDef {
	r.def.AdminOf = append(r.def.AdminOf, roles...)
	return r
}

func (r *RoleDef) PreviousName(name string) *RoleDef {
	r.def.PreviousName = name
	return r
}

func Collation(name string) *CollationDef {
	return CollationInSchema("", name)
}

func CollationInSchema(schema, name string) *CollationDef {
	collation := &pgschema.Collation{Schema: schema, Name: name}
	registerCollation(collation)
	return &CollationDef{def: collation}
}

func CollationFrom(name, existing string) *CollationDef {
	return CollationFromInSchema("", name, existing)
}

func CollationFromInSchema(schema, name, existing string) *CollationDef {
	collation := &pgschema.Collation{Schema: schema, Name: name, From: existing}
	registerCollation(collation)
	return &CollationDef{def: collation}
}

func (c *CollationDef) Provider(provider CollationProvider) *CollationDef {
	c.def.Provider = string(provider)
	return c
}

func (c *CollationDef) Locale(locale string) *CollationDef {
	c.def.Locale = locale
	return c
}

func (c *CollationDef) LCCollate(locale string) *CollationDef {
	c.def.LCCollate = locale
	return c
}

func (c *CollationDef) LCType(locale string) *CollationDef {
	c.def.LCType = locale
	return c
}

func (c *CollationDef) Deterministic(value bool) *CollationDef {
	c.def.Deterministic = &value
	return c
}

func (c *CollationDef) Rules(rules string) *CollationDef {
	c.def.Rules = rules
	return c
}

func (c *CollationDef) Version(version string) *CollationDef {
	c.def.Version = version
	return c
}

func (c *CollationDef) PreviousName(name string) *CollationDef {
	c.def.PreviousName = name
	return c
}

func Grant(privileges ...Privilege) *GrantDef {
	grant := &pgschema.Grant{}
	def := &GrantDef{def: grant}
	def.Privileges(privileges...)
	registerGrant(grant)
	return def
}

func (g *GrantDef) Privileges(privileges ...Privilege) *GrantDef {
	for _, privilege := range privileges {
		g.def.Privileges = append(g.def.Privileges, pgschema.GrantPrivilege{Name: string(privilege)})
	}
	return g
}

func (g *GrantDef) Columns(privilege Privilege, columns ...string) *GrantDef {
	g.def.Privileges = append(g.def.Privileges, pgschema.GrantPrivilege{
		Name:    string(privilege),
		Columns: append([]string(nil), columns...),
	})
	return g
}

func (g *GrantDef) OnTable(name string) *GrantDef {
	g.def.Target = pgschema.GrantTarget{Type: "table", Name: name}
	return g
}

func (g *GrantDef) OnSequence(name string) *GrantDef {
	g.def.Target = pgschema.GrantTarget{Type: "sequence", Name: name}
	return g
}

func (g *GrantDef) OnSchema(name string) *GrantDef {
	g.def.Target = pgschema.GrantTarget{Type: "schema", Name: name}
	return g
}

func (g *GrantDef) OnFunction(name string) *GrantDef {
	g.def.Target = pgschema.GrantTarget{Type: "function", Name: name}
	return g
}

func (g *GrantDef) OnType(name string) *GrantDef {
	g.def.Target = pgschema.GrantTarget{Type: "type", Name: name}
	return g
}

func (g *GrantDef) OnDatabase(name string) *GrantDef {
	g.def.Target = pgschema.GrantTarget{Type: "database", Name: name}
	return g
}

func (g *GrantDef) OnAllTablesInSchema(schema string) *GrantDef {
	g.def.Target = pgschema.GrantTarget{Type: "table", Schema: schema, AllInSchema: true}
	return g
}

func (g *GrantDef) OnAllSequencesInSchema(schema string) *GrantDef {
	g.def.Target = pgschema.GrantTarget{Type: "sequence", Schema: schema, AllInSchema: true}
	return g
}

func (g *GrantDef) OnAllFunctionsInSchema(schema string) *GrantDef {
	g.def.Target = pgschema.GrantTarget{Type: "function", Schema: schema, AllInSchema: true}
	return g
}

func (g *GrantDef) To(roles ...string) *GrantDef {
	g.def.Grantees = append(g.def.Grantees, roles...)
	return g
}

func (g *GrantDef) WithGrantOption() *GrantDef {
	g.def.GrantOption = true
	return g
}

func EnumType(name string, values ...string) *EnumDef {
	return EnumTypeInSchema("", name, values...)
}

func EnumTypeInSchema(schema, name string, values ...string) *EnumDef {
	enum := pgschema.Enum{Schema: schema, Name: name, Values: append([]string(nil), values...)}
	registerEnum(enum)
	return &EnumDef{def: enum}
}

func EnumColumn(name string, enum *EnumDef) *Column {
	return column(name, enum.TypeName())
}

func Sequence(name string, options ...SequenceOptions) *SequenceDef {
	return SequenceInSchema("", name, options...)
}

func SequenceInSchema(schema, name string, options ...SequenceOptions) *SequenceDef {
	seq := pgschema.Sequence{Schema: schema, Name: name}
	if len(options) > 1 {
		panic("sequence expects either no options or a single SequenceOptions")
	}
	if len(options) == 1 {
		opts := options[0]
		seq.Increment = opts.Increment
		seq.MinValue = opts.MinValue
		seq.MaxValue = opts.MaxValue
		seq.StartWith = opts.StartWith
		seq.Cache = opts.Cache
		seq.Cycle = opts.Cycle
		seq.OwnedBy = opts.OwnedBy
	}
	registerSequence(seq)
	return &SequenceDef{def: seq}
}

func (e *EnumDef) TypeName() string {
	if e.def.Schema == "" {
		return e.def.Name
	}
	return strings.Join([]string{e.def.Schema, e.def.Name}, ".")
}

func CompositeType(name string, attributes ...*CompositeAttributeDef) *CompositeTypeDef {
	return CompositeTypeInSchema("", name, attributes...)
}

func CompositeTypeInSchema(schema, name string, attributes ...*CompositeAttributeDef) *CompositeTypeDef {
	attrs := make([]pgschema.CompositeAttribute, 0, len(attributes))
	for _, attribute := range attributes {
		attrs = append(attrs, attribute.def)
	}
	typ := pgschema.CompositeType{Schema: schema, Name: name, Attributes: attrs}
	registerCompositeType(typ)
	return &CompositeTypeDef{def: typ}
}

func CompositeAttribute(name string, typ any) *CompositeAttributeDef {
	return &CompositeAttributeDef{
		def: pgschema.CompositeAttribute{Name: name, Type: typeSQL(typ)},
	}
}

func CompositeField(column *Column) *CompositeAttributeDef {
	return &CompositeAttributeDef{
		def: pgschema.CompositeAttribute{Name: column.def.Name, Type: column.def.Type},
	}
}

func (c *CompositeTypeDef) TypeName() string {
	if c.def.Schema == "" {
		return c.def.Name
	}
	return strings.Join([]string{c.def.Schema, c.def.Name}, ".")
}

func Domain(name, baseType string) *DomainDef {
	return DomainInSchema("", name, baseType)
}

func DomainInSchema(schema, name, baseType string) *DomainDef {
	domain := &pgschema.Domain{Schema: schema, Name: name, BaseType: baseType}
	registerDomain(domain)
	return &DomainDef{def: domain}
}

func (d *DomainDef) Default(expression string) *DomainDef {
	d.def.Default = expression
	return d
}

func (d *DomainDef) NotNull() *DomainDef {
	d.def.NotNull = true
	return d
}

func (d *DomainDef) Check(expression string) *DomainDef {
	d.def.Check = expression
	return d
}

func (d *DomainDef) TypeName() string {
	if d.def.Schema == "" {
		return d.def.Name
	}
	return strings.Join([]string{d.def.Schema, d.def.Name}, ".")
}

func Function(name string, returnTypeAndBody ...string) *FunctionDef {
	return FunctionInSchema("", name, returnTypeAndBody...)
}

func FunctionInSchema(schema, name string, returnTypeAndBody ...string) *FunctionDef {
	if len(returnTypeAndBody) != 0 && len(returnTypeAndBody) != 2 {
		panic("function expects either name only or name, return type, body")
	}
	function := &pgschema.Function{
		Schema:   schema,
		Name:     name,
		Language: "sql",
	}
	if len(returnTypeAndBody) == 2 {
		function.ReturnType = returnTypeAndBody[0]
		function.Body = returnTypeAndBody[1]
	}
	registerFunction(function)
	return &FunctionDef{def: function}
}

func SQLFunction(name string) *FunctionDef {
	return Function(name).Language("sql")
}

func SQLFunctionInSchema(schema, name string) *FunctionDef {
	return FunctionInSchema(schema, name).Language("sql")
}

func PLpgSQLFunction(name string) *FunctionDef {
	return Function(name).Language("plpgsql")
}

func PLpgSQLFunctionInSchema(schema, name string) *FunctionDef {
	return FunctionInSchema(schema, name).Language("plpgsql")
}

func FunctionArg(name string, typ any) *FunctionArgumentDef {
	return &FunctionArgumentDef{
		def: pgschema.FunctionArgument{Name: name, Type: typeSQL(typ)},
	}
}

func FunctionArgType(typ any) *FunctionArgumentDef {
	return &FunctionArgumentDef{
		def: pgschema.FunctionArgument{Type: typeSQL(typ)},
	}
}

func (f *FunctionDef) Returns(returnType any) *FunctionDef {
	f.def.ReturnType = typeSQL(returnType)
	return f
}

func (f *FunctionDef) Body(body any) *FunctionDef {
	f.def.Body = expressionSQL(body)
	return f
}

func (f *FunctionDef) Args(args ...*FunctionArgumentDef) *FunctionDef {
	for _, arg := range args {
		f.def.Arguments = append(f.def.Arguments, arg.def)
	}
	return f
}

func (f *FunctionDef) Language(language string) *FunctionDef {
	f.def.Language = language
	return f
}

func (f *FunctionDef) Immutable() *FunctionDef {
	f.def.Volatility = "IMMUTABLE"
	return f
}

func (f *FunctionDef) Stable() *FunctionDef {
	f.def.Volatility = "STABLE"
	return f
}

func (f *FunctionDef) Volatile() *FunctionDef {
	f.def.Volatility = "VOLATILE"
	return f
}

func (f *FunctionDef) Strict() *FunctionDef {
	f.def.Strict = new(true)
	return f
}

func (f *FunctionDef) CalledOnNullInput() *FunctionDef {
	f.def.Strict = new(false)
	return f
}

func (f *FunctionDef) SecurityDefiner() *FunctionDef {
	f.def.SecurityDefiner = true
	return f
}

func (f *FunctionDef) ParallelSafe() *FunctionDef {
	f.def.Parallel = "SAFE"
	return f
}

func (f *FunctionDef) ParallelRestricted() *FunctionDef {
	f.def.Parallel = "RESTRICTED"
	return f
}

func (f *FunctionDef) ParallelUnsafe() *FunctionDef {
	f.def.Parallel = "UNSAFE"
	return f
}

func (f *FunctionDef) Cost(cost float64) *FunctionDef {
	f.def.Cost = &cost
	return f
}

func (f *FunctionDef) Rows(rows int64) *FunctionDef {
	f.def.Rows = &rows
	return f
}

func (f *FunctionDef) Set(key, value string) *FunctionDef {
	if f.def.Configuration == nil {
		f.def.Configuration = map[string]string{}
	}
	f.def.Configuration[key] = value
	return f
}

func (f *FunctionDef) Comment(text string) *FunctionDef {
	f.def.Comment = text
	return f
}

func (f *FunctionDef) PreviousName(name string) *FunctionDef {
	f.def.PreviousName = name
	return f
}

func (a *FunctionArgumentDef) In() *FunctionArgumentDef {
	a.def.Mode = "IN"
	return a
}

func (a *FunctionArgumentDef) Out() *FunctionArgumentDef {
	a.def.Mode = "OUT"
	return a
}

func (a *FunctionArgumentDef) InOut() *FunctionArgumentDef {
	a.def.Mode = "INOUT"
	return a
}

func (a *FunctionArgumentDef) Variadic() *FunctionArgumentDef {
	a.def.Mode = "VARIADIC"
	return a
}

func (a *FunctionArgumentDef) Default(expression string) *FunctionArgumentDef {
	a.def.Default = expression
	return a
}

func Trigger(name, target, function string) *TriggerDef {
	trigger := &pgschema.Trigger{Name: name, Target: target, Function: function}
	registerTrigger(trigger)
	return &TriggerDef{def: trigger}
}

func (t *TriggerDef) Before() *TriggerDef {
	t.def.Timing = "BEFORE"
	return t
}

func (t *TriggerDef) After() *TriggerDef {
	t.def.Timing = "AFTER"
	return t
}

func (t *TriggerDef) InsteadOf() *TriggerDef {
	t.def.Timing = "INSTEAD OF"
	return t
}

func (t *TriggerDef) Insert() *TriggerDef {
	t.def.Events = append(t.def.Events, "INSERT")
	return t
}

func (t *TriggerDef) Update() *TriggerDef {
	t.def.Events = append(t.def.Events, "UPDATE")
	return t
}

func (t *TriggerDef) UpdateOf(columns ...string) *TriggerDef {
	t.def.Events = append(t.def.Events, "UPDATE")
	t.def.Columns = append(t.def.Columns, columns...)
	return t
}

func (t *TriggerDef) Delete() *TriggerDef {
	t.def.Events = append(t.def.Events, "DELETE")
	return t
}

func (t *TriggerDef) Truncate() *TriggerDef {
	t.def.Events = append(t.def.Events, "TRUNCATE")
	return t
}

func (t *TriggerDef) ForEachRow() *TriggerDef {
	t.def.Level = "ROW"
	return t
}

func (t *TriggerDef) ForEachStatement() *TriggerDef {
	t.def.Level = "STATEMENT"
	return t
}

func (t *TriggerDef) When(expression string) *TriggerDef {
	t.def.When = expression
	return t
}

func (t *TriggerDef) Args(args ...string) *TriggerDef {
	t.def.Arguments = append(t.def.Arguments, args...)
	return t
}

func (t *TriggerDef) Constraint() *TriggerDef {
	t.def.Constraint = true
	return t
}

func (t *TriggerDef) From(table string) *TriggerDef {
	t.def.ReferencedTable = table
	return t
}

func (t *TriggerDef) Deferrable() *TriggerDef {
	t.def.Deferrable = true
	return t
}

func (t *TriggerDef) InitiallyDeferred() *TriggerDef {
	t.def.Deferrable = true
	t.def.Initially = "DEFERRED"
	return t
}

func (t *TriggerDef) InitiallyImmediate() *TriggerDef {
	t.def.Deferrable = true
	t.def.Initially = "IMMEDIATE"
	return t
}

func (t *TriggerDef) PreviousName(name string) *TriggerDef {
	t.def.PreviousName = name
	return t
}

func (t *TriggerDef) Comment(text string) *TriggerDef {
	t.def.Comment = text
	return t
}

func Policy(name, table string) *PolicyDef {
	policy := &pgschema.Policy{Name: name, Table: table}
	registerPolicy(policy)
	return &PolicyDef{def: policy}
}

func (p *PolicyDef) Permissive() *PolicyDef {
	p.def.Mode = "PERMISSIVE"
	return p
}

func (p *PolicyDef) Restrictive() *PolicyDef {
	p.def.Mode = "RESTRICTIVE"
	return p
}

func (p *PolicyDef) All() *PolicyDef {
	p.def.Command = "ALL"
	return p
}

func (p *PolicyDef) Select() *PolicyDef {
	p.def.Command = "SELECT"
	return p
}

func (p *PolicyDef) Insert() *PolicyDef {
	p.def.Command = "INSERT"
	return p
}

func (p *PolicyDef) Update() *PolicyDef {
	p.def.Command = "UPDATE"
	return p
}

func (p *PolicyDef) Delete() *PolicyDef {
	p.def.Command = "DELETE"
	return p
}

func (p *PolicyDef) To(roles ...string) *PolicyDef {
	p.def.Roles = append(p.def.Roles, roles...)
	return p
}

func (p *PolicyDef) Using(expression string) *PolicyDef {
	p.def.Using = expression
	return p
}

func (p *PolicyDef) WithCheck(expression string) *PolicyDef {
	p.def.WithCheck = expression
	return p
}

func (p *PolicyDef) PreviousName(name string) *PolicyDef {
	p.def.PreviousName = name
	return p
}

type ViewDef struct {
	def *pgschema.View
}

func View(name string, query ...string) *ViewDef {
	return ViewInSchema("", name, query...)
}

func ViewInSchema(schema, name string, query ...string) *ViewDef {
	if len(query) > 1 {
		panic("view expects either no query or a single query")
	}
	view := &pgschema.View{Schema: schema, Name: name}
	if len(query) == 1 {
		view.Query = query[0]
	}
	registerView(view)
	return &ViewDef{def: view}
}

func (v *ViewDef) As(query any) *ViewDef {
	v.def.Query = expressionSQL(query)
	return v
}

func (v *ViewDef) Comment(text string) *ViewDef {
	v.def.Comment = text
	return v
}

func (v *ViewDef) CheckOption(option ViewCheckOption) *ViewDef {
	v.def.CheckOption = string(option)
	return v
}

func (v *ViewDef) SecurityBarrier() *ViewDef {
	v.def.SecurityBarrier = true
	return v
}

func (v *ViewDef) SecurityInvoker() *ViewDef {
	v.def.SecurityInvoker = true
	return v
}

func (v *ViewDef) Columns(aliases ...string) *ViewDef {
	v.def.ColumnAliases = append([]string(nil), aliases...)
	return v
}

func (v *ViewDef) DependsOn(tables ...string) *ViewDef {
	v.def.DependsOn = append([]string(nil), tables...)
	return v
}

func (v *ViewDef) TypeName() string {
	if v.def.Schema == "" {
		return v.def.Name
	}
	return strings.Join([]string{v.def.Schema, v.def.Name}, ".")
}

type MaterializedViewDef struct {
	def *pgschema.MaterializedView
}

func MaterializedView(name string, query ...string) *MaterializedViewDef {
	return MaterializedViewInSchema("", name, query...)
}

func MaterializedViewInSchema(schema, name string, query ...string) *MaterializedViewDef {
	if len(query) > 1 {
		panic("materialized view expects either no query or a single query")
	}
	mv := &pgschema.MaterializedView{Schema: schema, Name: name}
	if len(query) == 1 {
		mv.Query = query[0]
	}
	registerMaterializedView(mv)
	return &MaterializedViewDef{def: mv}
}

func (m *MaterializedViewDef) As(query any) *MaterializedViewDef {
	m.def.Query = expressionSQL(query)
	return m
}

func (m *MaterializedViewDef) Comment(text string) *MaterializedViewDef {
	m.def.Comment = text
	return m
}

func (m *MaterializedViewDef) Tablespace(name string) *MaterializedViewDef {
	m.def.Tablespace = name
	return m
}

func (m *MaterializedViewDef) With(key, value string) *MaterializedViewDef {
	if m.def.With == nil {
		m.def.With = make(map[string]string)
	}
	m.def.With[key] = value
	return m
}

func (m *MaterializedViewDef) NoData() *MaterializedViewDef {
	m.def.NoData = true
	return m
}

func (m *MaterializedViewDef) Columns(aliases ...string) *MaterializedViewDef {
	m.def.ColumnAliases = append([]string(nil), aliases...)
	return m
}

func (m *MaterializedViewDef) DependsOn(tables ...string) *MaterializedViewDef {
	m.def.DependsOn = append([]string(nil), tables...)
	return m
}

func (m *MaterializedViewDef) TypeName() string {
	if m.def.Schema == "" {
		return m.def.Name
	}
	return strings.Join([]string{m.def.Schema, m.def.Name}, ".")
}
