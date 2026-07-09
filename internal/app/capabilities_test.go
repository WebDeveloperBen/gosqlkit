package app

import (
	"strings"
	"sync"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/kit"
)

var registerUnsupportedProviderOnce sync.Once

type unsupportedProvider struct{}

func (unsupportedProvider) Dialect() kit.DialectInfo {
	return kit.DialectInfo{
		Name:    "unsupported-test",
		Aliases: []string{"unsupported-test-alias"},
	}
}

func (unsupportedProvider) RenderSQL() (string, error) {
	return "", nil
}

func (unsupportedProvider) SnapshotJSON() ([]byte, error) {
	return []byte("{}\n"), nil
}

func registerUnsupportedProvider() {
	registerUnsupportedProviderOnce.Do(func() {
		kit.Register(unsupportedProvider{})
	})
}

func unsupportedConfig(t *testing.T) *Config {
	t.Helper()
	return &Config{
		Dialect: "unsupported-test",
		rootDir: t.TempDir(),
		Migrations: MigrationSpec{
			Runner: migrate.RunnerGoose,
			Dir:    "migrations",
		},
	}
}

func TestGenerateRejectsUnsupportedRenderCapability(t *testing.T) {
	registerUnsupportedProvider()

	_, err := Generate(GenerateOptions{
		Package: "./examples/basic/schema",
		Dialect: "unsupported-test",
		Root:    mustModuleDir(t),
	})
	if err == nil || !strings.Contains(err.Error(), `dialect "unsupported-test" does not support SQL generation`) {
		t.Fatalf("expected unsupported SQL generation error, got %v", err)
	}
}

func TestSnapshotRejectsUnsupportedSnapshotCapability(t *testing.T) {
	registerUnsupportedProvider()

	_, err := Snapshot(SnapshotOptions{
		Package: "./examples/basic/schema",
		Dialect: "unsupported-test",
		Root:    mustModuleDir(t),
	})
	if err == nil || !strings.Contains(err.Error(), `dialect "unsupported-test" does not support snapshot generation`) {
		t.Fatalf("expected unsupported snapshot error, got %v", err)
	}
}

func TestInspectRejectsUnsupportedInspectCapabilityBeforeURL(t *testing.T) {
	registerUnsupportedProvider()

	_, err := InspectWithConfig(unsupportedConfig(t), InspectOptions{})
	if err == nil || !strings.Contains(err.Error(), `dialect "unsupported-test" does not support database inspection`) {
		t.Fatalf("expected unsupported inspect error, got %v", err)
	}
}

func TestDriftCheckRejectsUnsupportedDriftCapabilityBeforeURL(t *testing.T) {
	registerUnsupportedProvider()

	_, err := DriftCheckWithConfig(unsupportedConfig(t), DriftCheckOptions{})
	if err == nil || !strings.Contains(err.Error(), `dialect "unsupported-test" does not support drift checking`) {
		t.Fatalf("expected unsupported drift error, got %v", err)
	}
}

func TestMigratePlanRejectsUnsupportedMigrationPlanCapabilityBeforeDirScan(t *testing.T) {
	registerUnsupportedProvider()

	_, err := MigratePlanWithConfig(unsupportedConfig(t), MigratePlanOptions{})
	if err == nil || !strings.Contains(err.Error(), `dialect "unsupported-test" does not support migration planning`) {
		t.Fatalf("expected unsupported migration planning error, got %v", err)
	}
}

func TestMigrateApplyRejectsUnsupportedApplyCapabilityBeforeURL(t *testing.T) {
	registerUnsupportedProvider()

	_, err := MigrateApplyWithConfig(unsupportedConfig(t), MigrateApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), `dialect "unsupported-test" does not support migration apply`) {
		t.Fatalf("expected unsupported migration apply error, got %v", err)
	}
}

func TestMigrateCheckRejectsUnsupportedSandboxReplayWhenSandboxURLIsProvided(t *testing.T) {
	registerUnsupportedProvider()

	_, err := MigrateCheckWithConfig(unsupportedConfig(t), MigrateCheckOptions{
		SandboxURL: "postgres://localhost/app",
	})
	if err == nil || !strings.Contains(err.Error(), `dialect "unsupported-test" does not support migration sandbox replay`) {
		t.Fatalf("expected unsupported sandbox replay error, got %v", err)
	}
}
