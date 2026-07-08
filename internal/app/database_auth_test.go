package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSplitCommandLine(t *testing.T) {
	got, err := splitCommandLine(`az account get-access-token --query "accessToken" --resource-type 'oss-rdbms' escaped\ value`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"az", "account", "get-access-token", "--query", "accessToken", "--resource-type", "oss-rdbms", "escaped value"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split command line = %#v, want %#v", got, want)
	}
}

func TestSplitCommandLineRejectsUnterminatedQuote(t *testing.T) {
	_, err := splitCommandLine(`token-command "unterminated`)
	if err == nil || !strings.Contains(err.Error(), "unterminated quoted string") {
		t.Fatalf("expected unterminated quote error, got %v", err)
	}
}

func TestCommandPasswordProviderRunsCommand(t *testing.T) {
	command := writeTokenCommand(t, "secret-token", 0)

	password, err := (commandPasswordProvider{command: command}).Password(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if password != "secret-token" {
		t.Fatalf("password = %q", password)
	}
}

func TestCommandPasswordProviderRedactsTokenOutputOnFailure(t *testing.T) {
	command := writeTokenCommand(t, "secret-token", 1)

	_, err := (commandPasswordProvider{command: command}).Password(context.Background())
	if err == nil {
		t.Fatal("expected command error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("token leaked in error: %v", err)
	}
}

func writeTokenCommand(t *testing.T, token string, exitCode int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token.sh")
	content := "#!/bin/sh\nprintf '%s\\n' '" + token + "'\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// #nosec G302 -- test helper intentionally creates an executable token-command script.
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
