package kit_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/webdeveloperben/gosqlkit/kit"
)

var registerFakeOnce sync.Once

type fakeProvider struct{}

func (fakeProvider) Dialect() kit.DialectInfo {
	return kit.DialectInfo{
		Name:    "fake-test",
		Aliases: []string{"fake-test-alias"},
		Capabilities: kit.Capabilities{
			Tables:    true,
			Snapshots: true,
		},
	}
}

func (fakeProvider) RenderSQL() (string, error) {
	return "CREATE TABLE fake (id integer);\n", nil
}

func (fakeProvider) SnapshotJSON() ([]byte, error) {
	return []byte("{\"dialect\":\"fake\"}\n"), nil
}

func registerFakeProvider() {
	registerFakeOnce.Do(func() {
		kit.Register(fakeProvider{})
	})
}

func TestRegisterRenderAndSnapshot(t *testing.T) {
	registerFakeProvider()

	sql, err := kit.RenderSQL("fake-test")
	if err != nil {
		t.Fatal(err)
	}
	if sql != "CREATE TABLE fake (id integer);\n" {
		t.Fatalf("unexpected SQL %q", sql)
	}

	snapshot, err := kit.SnapshotJSON("fake-test")
	if err != nil {
		t.Fatal(err)
	}
	if string(snapshot) != "{\"dialect\":\"fake\"}\n" {
		t.Fatalf("unexpected snapshot %q", snapshot)
	}
}

func TestRegisterAliasAndInfo(t *testing.T) {
	registerFakeProvider()

	sql, err := kit.RenderSQL("FAKE-TEST-ALIAS")
	if err != nil {
		t.Fatal(err)
	}
	if sql != "CREATE TABLE fake (id integer);\n" {
		t.Fatalf("unexpected SQL %q", sql)
	}

	info, err := kit.DialectInfoFor("fake-test-alias")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "fake-test" {
		t.Fatalf("unexpected dialect name %q", info.Name)
	}
	if len(info.Aliases) != 1 || info.Aliases[0] != "fake-test-alias" {
		t.Fatalf("unexpected aliases %#v", info.Aliases)
	}
	if !info.Capabilities.Tables || !info.Capabilities.Snapshots {
		t.Fatalf("unexpected capabilities %#v", info.Capabilities)
	}
}

func TestUnknownDialectListsRegisteredDialects(t *testing.T) {
	registerFakeProvider()

	_, err := kit.RenderSQL("missing-dialect")
	if err == nil || !strings.Contains(err.Error(), `unknown dialect "missing-dialect"`) {
		t.Fatalf("expected unknown dialect error, got %v", err)
	}
}
