package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func runWithRecover(t *testing.T, args []string) (code int, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(*ExitPanic); ok {
				code = ep.Code
				return
			}
			t.Fatalf("unexpected panic in Run: %v", r)
		}
	}()
	code, err = Run(args)
	return code, err
}

func TestRunNoArgsPrintsHelp(t *testing.T) {
	code, err := runWithRecover(t, nil)
	if err != nil {
		t.Fatalf("Run(nil) returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("Run(nil) returned code %d, want 0", code)
	}
}

func TestRunVersionExitsZero(t *testing.T) {
	code, err := runWithRecover(t, []string{"version"})
	if err != nil {
		t.Fatalf("Run(version) returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("Run(version) returned code %d, want 0", code)
	}
}

func TestRunUnknownCommandExitsTwo(t *testing.T) {
	code, _ := runWithRecover(t, []string{"unknown-cmd"})
	if code != 2 {
		t.Fatalf("Run(unknown-cmd) returned code %d, want 2", code)
	}
}

func TestRunGenerateCheckRequiresOut(t *testing.T) {
	var stderr bytes.Buffer
	code, err := run([]string{"generate", "--check", "./examples/basic/schema"}, nil, &stderr)
	if code != 1 {
		t.Fatalf("run(generate --check) returned code %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "--check requires --out") {
		t.Fatalf("expected --check error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "gosqlkit: error:") {
		t.Fatalf("expected error output, got %q", stderr.String())
	}
}

func TestExitErrorWraps(t *testing.T) {
	inner := errors.New("boom")
	got := Exit(7, inner)

	var exitErr *ExitError
	if !errors.As(got, &exitErr) || exitErr.Code != 7 {
		t.Errorf("Exit(7, err) lost code: got %T, %v", got, got)
	}
}
