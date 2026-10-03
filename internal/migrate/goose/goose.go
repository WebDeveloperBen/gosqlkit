package goose

import (
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/sqlsplit"
)

type Renderer struct{}

func (Renderer) Runner() string {
	return migrate.RunnerGoose
}

func (Renderer) Render(plan migrate.Plan) ([]migrate.File, error) {
	stem, err := migrate.FileStem(plan)
	if err != nil {
		return nil, err
	}

	meta, err := migrate.PlanMetadata(plan)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString(meta)
	b.WriteString("\n\n-- +goose Up\n")
	migrate.WriteSQLSection(&b, statementBlocks(plan.Dialect, migrate.SQLStatements(plan.UpStatements, plan.UpSQL)))
	if plan.DownStatements != nil || plan.DownSQL != nil {
		b.WriteString("\n-- +goose Down\n")
		migrate.WriteSQLSection(&b, statementBlocks(plan.Dialect, migrate.SQLStatements(plan.DownStatements, plan.DownSQL)))
	}

	return []migrate.File{{
		Name:    stem + ".sql",
		Content: b.String(),
	}}, nil
}

func statementBlocks(dialect string, statements []string) []string {
	if strings.EqualFold(strings.TrimSpace(dialect), "sqlite") || strings.EqualFold(strings.TrimSpace(dialect), "sqlite3") {
		var split []string
		for _, statement := range statements {
			split = append(split, sqlsplit.StatementsWithTriggers(statement)...)
		}
		statements = split
	}
	out := make([]string, 0, len(statements))
	for _, statement := range statements {
		if !sqlsplit.IsTriggerStatement(statement) {
			out = append(out, statement)
			continue
		}
		out = append(out, "-- +goose StatementBegin\n"+strings.TrimSpace(statement)+"\n-- +goose StatementEnd")
	}
	return out
}
