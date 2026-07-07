package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	RunnerGoose     = "goose"
	MetadataVersion = 1
)

type Change struct {
	Op           string      `json:"op"`
	Summary      string      `json:"summary,omitempty"`
	Object       ObjectRef   `json:"object"`
	Risks        []string    `json:"risks,omitempty"`
	Dependencies []ObjectRef `json:"dependencies,omitempty"`
	Reversible   bool        `json:"reversible,omitempty"`
}

type ObjectRef struct {
	Kind string `json:"kind"`
	Key  string `json:"key"`
}

type Statement struct {
	SQL string `json:"sql"`
}

type Plan struct {
	CreatedAt      time.Time
	Name           string
	Dialect        string
	FromSnapshotID string
	ToSnapshotID   string
	TargetSnapshot string
	Changes        []Change
	UpStatements   []Statement
	DownStatements []Statement
	UpSQL          []string
	DownSQL        []string
}

type File struct {
	Name    string
	Content string
}

type Renderer interface {
	Runner() string
	Render(plan Plan) ([]File, error)
}

type Metadata struct {
	Dialect        string          `json:"dialect"`
	FromSnapshotID string          `json:"fromSnapshotId,omitempty"`
	ToSnapshotID   string          `json:"toSnapshotId,omitempty"`
	TargetSnapshot json.RawMessage `json:"targetSnapshot,omitempty"`
	CreatedAt      string          `json:"createdAt"`
	Changes        []Change        `json:"changes"`
	Version        int             `json:"version"`
}

type Migration struct {
	Path     string
	Name     string
	Content  string
	Metadata Metadata
}

func NormaliseRunner(runner string) string {
	if runner == "" {
		return RunnerGoose
	}
	return strings.TrimSpace(strings.ToLower(runner))
}

func PlanMetadata(plan Plan) (string, error) {
	createdAt := plan.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	changes := plan.Changes
	if changes == nil {
		changes = []Change{}
	}
	var targetSnapshot json.RawMessage
	if plan.TargetSnapshot != "" {
		targetSnapshot = json.RawMessage(plan.TargetSnapshot)
		if !json.Valid(targetSnapshot) {
			return "", errors.New("target snapshot must be valid JSON")
		}
	}

	data, err := json.MarshalIndent(Metadata{
		Version:        MetadataVersion,
		Dialect:        plan.Dialect,
		FromSnapshotID: plan.FromSnapshotID,
		ToSnapshotID:   plan.ToSnapshotID,
		TargetSnapshot: targetSnapshot,
		CreatedAt:      createdAt.UTC().Format(time.RFC3339),
		Changes:        changes,
	}, "", "  ")
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		lines[i] = "-- " + line
	}
	return "-- +gosqlkit Meta\n" + strings.Join(lines, "\n"), nil
}

func ParseMetadata(content string) (Metadata, error) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "-- +gosqlkit Meta" {
			continue
		}
		return parseMetadataLines(lines[i+1:])
	}
	return Metadata{}, errors.New("missing gosqlkit metadata block")
}

func ScanDir(dir string) ([]Migration, error) {
	// #nosec G304 -- migration directory is resolved from config or CLI input and intentionally scanned.
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Migration{}, nil
		}
		return nil, err
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		if err := validateMigrationFileName(entry.Name()); err != nil {
			return nil, err
		}

		path := filepath.Join(dir, entry.Name())
		// #nosec G304 -- migration file path comes from scanning the configured migration directory.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		meta, err := ParseMetadata(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		migrations = append(migrations, Migration{
			Name:     entry.Name(),
			Path:     path,
			Content:  string(data),
			Metadata: meta,
		})
	}

	sort.SliceStable(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})
	if err := validateLineage(migrations); err != nil {
		return nil, err
	}
	return migrations, nil
}

func parseMetadataLines(lines []string) (Metadata, error) {
	var jsonLines []string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			break
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			break
		}
		comment := strings.TrimPrefix(strings.TrimSpace(line), "--")
		comment = strings.TrimPrefix(comment, " ")
		if strings.HasPrefix(strings.TrimSpace(comment), "+") {
			break
		}
		jsonLines = append(jsonLines, comment)
	}
	if len(jsonLines) == 0 {
		return Metadata{}, errors.New("empty gosqlkit metadata block")
	}

	var meta Metadata
	if err := json.Unmarshal([]byte(strings.Join(jsonLines, "\n")), &meta); err != nil {
		return Metadata{}, fmt.Errorf("parse gosqlkit metadata: %w", err)
	}
	if meta.Version != MetadataVersion {
		return Metadata{}, fmt.Errorf("unsupported gosqlkit metadata version %d", meta.Version)
	}
	if meta.CreatedAt == "" {
		return Metadata{}, errors.New("gosqlkit metadata missing createdAt")
	}
	if _, err := time.Parse(time.RFC3339, meta.CreatedAt); err != nil {
		return Metadata{}, fmt.Errorf("parse gosqlkit metadata createdAt: %w", err)
	}
	if err := validateTargetSnapshot(meta); err != nil {
		return Metadata{}, err
	}
	if meta.Changes == nil {
		meta.Changes = []Change{}
	}
	return meta, nil
}

func validateTargetSnapshot(meta Metadata) error {
	if len(meta.TargetSnapshot) == 0 {
		return nil
	}
	var raw struct {
		SnapshotID string `json:"snapshotId"`
	}
	if err := json.Unmarshal(meta.TargetSnapshot, &raw); err != nil {
		return fmt.Errorf("parse targetSnapshot: %w", err)
	}
	if raw.SnapshotID == "" {
		return errors.New("targetSnapshot missing snapshotId")
	}
	if meta.ToSnapshotID != "" && raw.SnapshotID != meta.ToSnapshotID {
		return fmt.Errorf("targetSnapshot snapshotId %q does not match toSnapshotId %q",
			raw.SnapshotID, meta.ToSnapshotID)
	}
	return nil
}

func validateMigrationFileName(name string) error {
	if len(name) < len("20060102150405_a.sql") {
		return fmt.Errorf("migration file %q must start with YYYYMMDDHHMMSS_", name)
	}
	prefix := name[:14]
	if _, err := time.Parse("20060102150405", prefix); err != nil {
		return fmt.Errorf("migration file %q has invalid timestamp prefix: %w", name, err)
	}
	if name[14] != '_' {
		return fmt.Errorf("migration file %q must start with YYYYMMDDHHMMSS_", name)
	}
	return nil
}

func validateLineage(migrations []Migration) error {
	var previous Migration
	for i, migration := range migrations {
		if i == 0 {
			previous = migration
			continue
		}
		if previous.Metadata.ToSnapshotID != "" && migration.Metadata.FromSnapshotID != "" &&
			previous.Metadata.ToSnapshotID != migration.Metadata.FromSnapshotID {
			return fmt.Errorf("%s fromSnapshotId %q does not match previous toSnapshotId %q",
				migration.Path, migration.Metadata.FromSnapshotID, previous.Metadata.ToSnapshotID)
		}
		previous = migration
	}
	return nil
}
