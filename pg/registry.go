package pg

import (
	"sync"

	"github.com/webdeveloperben/pgkit/internal/ast"
	"github.com/webdeveloperben/pgkit/internal/render"
)

var registry = struct {
	tables []ast.Table
	sync.Mutex
}{}

func Schema() ast.Schema {
	registry.Lock()
	defer registry.Unlock()

	tables := append([]ast.Table(nil), registry.tables...)
	return ast.Schema{Tables: tables}
}

func Render() (string, error) {
	return render.Postgres(Schema())
}

func MustRender() string {
	sql, err := Render()
	if err != nil {
		panic(err)
	}
	return sql
}

func Reset() {
	registry.Lock()
	defer registry.Unlock()

	registry.tables = nil
}

func register(table ast.Table) {
	registry.Lock()
	defer registry.Unlock()

	registry.tables = append(registry.tables, table)
}
