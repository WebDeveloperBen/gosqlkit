package goose

import (
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
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
	migrate.WriteSQLSection(&b, migrate.SQLStatements(plan.UpStatements, plan.UpSQL))
	if plan.DownStatements != nil || plan.DownSQL != nil {
		b.WriteString("\n-- +goose Down\n")
		migrate.WriteSQLSection(&b, migrate.SQLStatements(plan.DownStatements, plan.DownSQL))
	}

	return []migrate.File{{
		Name:    stem + ".sql",
		Content: b.String(),
	}}, nil
}
