package goose

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
)

type Renderer struct{}

func (Renderer) Runner() string {
	return migrate.RunnerGoose
}

func (Renderer) Render(plan migrate.Plan) ([]migrate.File, error) {
	name, err := fileName(plan)
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
	writeSQLSection(&b, plan.UpSQL)
	if plan.DownSQL != nil {
		b.WriteString("\n-- +goose Down\n")
		writeSQLSection(&b, plan.DownSQL)
	}

	return []migrate.File{{
		Name:    name,
		Content: b.String(),
	}}, nil
}

func fileName(plan migrate.Plan) (string, error) {
	slug := slugify(plan.Name)
	if slug == "" {
		return "", errors.New("migration name must contain at least one letter or number")
	}

	createdAt := plan.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	return fmt.Sprintf("%s_%s.sql", createdAt.UTC().Format("20060102150405"), slug), nil
}

func writeSQLSection(b *strings.Builder, statements []string) {
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		b.WriteString(statement)
		if !strings.HasSuffix(statement, "\n") {
			b.WriteByte('\n')
		}
		if !strings.HasSuffix(statement, "\n\n") {
			b.WriteByte('\n')
		}
	}
}

func slugify(value string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}
