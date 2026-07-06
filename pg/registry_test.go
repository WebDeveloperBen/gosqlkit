package pg_test

import (
	"slices"
	"testing"

	"github.com/webdeveloperben/gosqlkit/kit"
	_ "github.com/webdeveloperben/gosqlkit/pg"
)

func TestProviderRegistersPostgresDialectMetadata(t *testing.T) {
	info, err := kit.DialectInfoFor("postgresql")
	if err != nil {
		t.Fatal(err)
	}

	if info.Name != "postgres" {
		t.Fatalf("unexpected dialect name %q", info.Name)
	}
	if !contains(info.Aliases, "pg") || !contains(info.Aliases, "postgresql") {
		t.Fatalf("expected pg and postgresql aliases, got %#v", info.Aliases)
	}
	if !info.Capabilities.Tables || !info.Capabilities.Schemas || !info.Capabilities.Extensions ||
		!info.Capabilities.Enums || !info.Capabilities.AdvancedIndexes || !info.Capabilities.Snapshots ||
		!info.Capabilities.Roles || !info.Capabilities.Functions || !info.Capabilities.Triggers ||
		!info.Capabilities.RLS {
		t.Fatalf("unexpected capabilities %#v", info.Capabilities)
	}
}

func contains(values []string, needle string) bool {
	return slices.Contains(values, needle)
}
