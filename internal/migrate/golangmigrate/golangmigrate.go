package golangmigrate

import (
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/sqlsplit"
)

type Renderer struct{}

func (Renderer) Runner() string {
	return migrate.RunnerGolangMigrate
}

func (Renderer) Render(plan migrate.Plan) ([]migrate.File, error) {
	stem, err := migrate.FileStem(plan)
	if err != nil {
		return nil, err
	}

	upStatements := splitStatements(migrate.SQLStatements(plan.UpStatements, plan.UpSQL))
	if len(upStatements) == 0 {
		upStatements = []string{""}
	}
	downStatements := splitStatements(migrate.SQLStatements(plan.DownStatements, plan.DownSQL))
	if len(upStatements) > 1 && len(downStatements) > 0 && len(downStatements) != len(upStatements) {
		return nil, fmt.Errorf("cannot safely pair %d down statements with %d split golang-migrate up statements", len(downStatements), len(upStatements))
	}

	files := make([]migrate.File, 0, len(upStatements)*2)
	for i, statement := range upStatements {
		chunkStem, err := chunkStem(stem, i, len(upStatements))
		if err != nil {
			return nil, err
		}
		meta, err := chunkMetadata(plan, i == len(upStatements)-1)
		if err != nil {
			return nil, err
		}
		var up strings.Builder
		up.WriteString(meta)
		up.WriteString("\n\n")
		migrate.WriteSQLSection(&up, []string{statement})
		files = append(files, migrate.File{
			Name:    chunkStem + ".up.sql",
			Content: up.String(),
		})
	}

	if plan.DownStatements != nil || plan.DownSQL != nil {
		for i := range upStatements {
			chunkStem, err := chunkStem(stem, i, len(upStatements))
			if err != nil {
				return nil, err
			}
			var down strings.Builder
			if len(upStatements) == 1 {
				migrate.WriteSQLSection(&down, downStatements)
			} else if len(downStatements) == len(upStatements) {
				migrate.WriteSQLSection(&down, []string{downStatements[len(downStatements)-1-i]})
			}
			files = append(files, migrate.File{
				Name:    chunkStem + ".down.sql",
				Content: down.String(),
			})
		}
	}
	return files, nil
}

func Validate(migrations []migrate.Migration) error {
	for _, migration := range migrations {
		if !strings.HasSuffix(migration.Name, ".up.sql") {
			return fmt.Errorf("%s: golang-migrate metadata must be stored in .up.sql files", migration.Path)
		}
	}
	return nil
}

func UpSQL(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "-- +gosqlkit Meta" {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				return strings.Join(lines[j+1:], "\n")
			}
			if !strings.HasPrefix(trimmed, "--") {
				return strings.Join(lines[j:], "\n")
			}
			comment := strings.TrimSpace(strings.TrimPrefix(trimmed, "--"))
			if strings.HasPrefix(comment, "+") {
				return strings.Join(lines[j:], "\n")
			}
		}
		return ""
	}
	return content
}

func splitStatements(statements []string) []string {
	out := make([]string, 0, len(statements))
	for _, statement := range statements {
		out = append(out, sqlsplit.Statements(statement)...)
	}
	return out
}

func chunkStem(stem string, index int, total int) (string, error) {
	if total == 1 {
		return stem, nil
	}
	if index >= 9999 {
		return "", fmt.Errorf("golang-migrate split migration has too many statements: %d", total)
	}
	timestamp, slug, ok := strings.Cut(stem, "_")
	if !ok {
		return "", fmt.Errorf("invalid migration stem %q", stem)
	}
	return fmt.Sprintf("%s%04d_%s", timestamp, index+1, slug), nil
}

func chunkMetadata(plan migrate.Plan, final bool) (string, error) {
	if final {
		return migrate.PlanMetadata(plan)
	}
	chunk := plan
	chunk.ToSnapshotID = ""
	chunk.TargetSnapshot = ""
	return migrate.PlanMetadata(chunk)
}
