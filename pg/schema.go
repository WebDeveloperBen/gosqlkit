package pg

import (
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

type NamespaceDef struct {
	def pgschema.Namespace
}

type ExtensionDef struct {
	def pgschema.Extension
}

type RoleDef struct {
	def *pgschema.Role
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

func Namespace(name string) *NamespaceDef {
	namespace := pgschema.Namespace{Name: name}
	registerNamespace(namespace)
	return &NamespaceDef{def: namespace}
}

func Extension(name string) *ExtensionDef {
	return ExtensionInSchema("", name)
}

func ExtensionInSchema(schema, name string) *ExtensionDef {
	extension := pgschema.Extension{Schema: schema, Name: name}
	registerExtension(extension)
	return &ExtensionDef{def: extension}
}

func Role(name string) *RoleDef {
	role := &pgschema.Role{Name: name}
	registerRole(role)
	return &RoleDef{def: role}
}

func (r *RoleDef) Login() *RoleDef {
	r.def.Login = roleBool(true)
	return r
}

func (r *RoleDef) NoLogin() *RoleDef {
	r.def.Login = roleBool(false)
	return r
}

func (r *RoleDef) Superuser() *RoleDef {
	r.def.Superuser = roleBool(true)
	return r
}

func (r *RoleDef) NoSuperuser() *RoleDef {
	r.def.Superuser = roleBool(false)
	return r
}

func (r *RoleDef) CreateDB() *RoleDef {
	r.def.CreateDB = roleBool(true)
	return r
}

func (r *RoleDef) NoCreateDB() *RoleDef {
	r.def.CreateDB = roleBool(false)
	return r
}

func (r *RoleDef) CreateRole() *RoleDef {
	r.def.CreateRole = roleBool(true)
	return r
}

func (r *RoleDef) NoCreateRole() *RoleDef {
	r.def.CreateRole = roleBool(false)
	return r
}

func (r *RoleDef) Inherit() *RoleDef {
	r.def.Inherit = roleBool(true)
	return r
}

func (r *RoleDef) NoInherit() *RoleDef {
	r.def.Inherit = roleBool(false)
	return r
}

func (r *RoleDef) Replication() *RoleDef {
	r.def.Replication = roleBool(true)
	return r
}

func (r *RoleDef) NoReplication() *RoleDef {
	r.def.Replication = roleBool(false)
	return r
}

func (r *RoleDef) BypassRLS() *RoleDef {
	r.def.BypassRLS = roleBool(true)
	return r
}

func (r *RoleDef) NoBypassRLS() *RoleDef {
	r.def.BypassRLS = roleBool(false)
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

func roleBool(value bool) *bool {
	return &value
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

func CompositeAttribute(name, typ string) *CompositeAttributeDef {
	return &CompositeAttributeDef{
		def: pgschema.CompositeAttribute{Name: name, Type: typ},
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

func Function(name, returnType, body string) *FunctionDef {
	return FunctionInSchema("", name, returnType, body)
}

func FunctionInSchema(schema, name, returnType, body string) *FunctionDef {
	function := &pgschema.Function{
		Schema:     schema,
		Name:       name,
		Language:   "sql",
		ReturnType: returnType,
		Body:       body,
	}
	registerFunction(function)
	return &FunctionDef{def: function}
}

func FunctionArg(name, typ string) *FunctionArgumentDef {
	return &FunctionArgumentDef{
		def: pgschema.FunctionArgument{Name: name, Type: typ},
	}
}

func FunctionArgType(typ string) *FunctionArgumentDef {
	return &FunctionArgumentDef{
		def: pgschema.FunctionArgument{Type: typ},
	}
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
	f.def.Strict = roleBool(true)
	return f
}

func (f *FunctionDef) CalledOnNullInput() *FunctionDef {
	f.def.Strict = roleBool(false)
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

type ViewDef struct {
	def *pgschema.View
}

func View(name, query string) *ViewDef {
	return ViewInSchema("", name, query)
}

func ViewInSchema(schema, name, query string) *ViewDef {
	view := &pgschema.View{Schema: schema, Name: name, Query: query}
	registerView(view)
	return &ViewDef{def: view}
}

func (v *ViewDef) Comment(text string) *ViewDef {
	v.def.Comment = text
	return v
}

func (v *ViewDef) CheckOption(option string) *ViewDef {
	v.def.CheckOption = option
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

func MaterializedView(name, query string) *MaterializedViewDef {
	return MaterializedViewInSchema("", name, query)
}

func MaterializedViewInSchema(schema, name, query string) *MaterializedViewDef {
	mv := &pgschema.MaterializedView{Schema: schema, Name: name, Query: query}
	registerMaterializedView(mv)
	return &MaterializedViewDef{def: mv}
}

func (m *MaterializedViewDef) Comment(text string) *MaterializedViewDef {
	m.def.Comment = text
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
