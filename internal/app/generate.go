package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GenerateOptions struct {
	Stdout  io.Writer
	Package string
	Dialect string
	Out     string
	Root    string
	Check   bool
}

type GenerateResult struct {
	Out     string `json:"out,omitempty"`
	Package string `json:"package"`
	Checked bool   `json:"checked"`
}

type SnapshotOptions struct {
	Stdout  io.Writer
	Package string
	Dialect string
	Out     string
	Root    string
	Check   bool
}

type SnapshotResult struct {
	Out     string `json:"out,omitempty"`
	Package string `json:"package"`
	Checked bool   `json:"checked"`
}

func Generate(opts GenerateOptions) (*GenerateResult, error) {
	if opts.Package == "" {
		return nil, errors.New("schema package is required")
	}
	if opts.Check && opts.Out == "" {
		return nil, errors.New("generate --check requires --out")
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}

	root, err := ResolveRoot(opts.Root)
	if err != nil {
		return nil, err
	}

	importPath, moduleDir, err := resolvePackage(root, opts.Package)
	if err != nil {
		return nil, err
	}

	sql, err := runPackageProgram(importPath, moduleDir, fmt.Sprintf(`sql, err := kit.RenderSQL(%q)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(sql)`, dialect(opts.Dialect)))
	if err != nil {
		return nil, err
	}

	out := ResolvePath(root, opts.Out)
	if opts.Check {
		if err := checkOutput(out, sql); err != nil {
			return nil, err
		}
		return &GenerateResult{Checked: true, Out: out, Package: importPath}, nil
	}
	if opts.Out != "" {
		if err := writeOutput(out, sql); err != nil {
			return nil, err
		}
		return &GenerateResult{Out: out, Package: importPath}, nil
	}

	if _, err := io.WriteString(opts.Stdout, sql); err != nil {
		return nil, err
	}
	return &GenerateResult{Package: importPath}, nil
}

func Snapshot(opts SnapshotOptions) (*SnapshotResult, error) {
	if opts.Package == "" {
		return nil, errors.New("schema package is required")
	}
	if opts.Check && opts.Out == "" {
		return nil, errors.New("snapshot --check requires --out")
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}

	root, err := ResolveRoot(opts.Root)
	if err != nil {
		return nil, err
	}

	importPath, moduleDir, err := resolvePackage(root, opts.Package)
	if err != nil {
		return nil, err
	}

	json, err := runPackageProgram(importPath, moduleDir, fmt.Sprintf(`data, err := kit.SnapshotJSON(%q)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}`, dialect(opts.Dialect)))
	if err != nil {
		return nil, err
	}

	out := ResolvePath(root, opts.Out)
	if opts.Check {
		if err := checkOutput(out, json); err != nil {
			return nil, err
		}
		return &SnapshotResult{Checked: true, Out: out, Package: importPath}, nil
	}
	if opts.Out != "" {
		if err := writeOutput(out, json); err != nil {
			return nil, err
		}
		return &SnapshotResult{Out: out, Package: importPath}, nil
	}

	if _, err := io.WriteString(opts.Stdout, json); err != nil {
		return nil, err
	}
	return &SnapshotResult{Package: importPath}, nil
}

func ResolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}

func ResolvePath(root, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func resolvePackage(root, pkg string) (importPath string, moduleDir string, err error) {
	// #nosec G204 -- pkg is a user-supplied Go package path passed to the Go tool by design.
	listCmd := exec.Command("go", "list", "-f", "{{.ImportPath}}", pkg)
	listCmd.Dir = root
	importOutput, err := listCmd.Output()
	if err != nil {
		return "", "", commandError("go list package", err)
	}

	moduleCmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
	moduleCmd.Dir = root
	moduleOutput, err := moduleCmd.Output()
	if err != nil {
		return "", "", commandError("go list module", err)
	}

	return strings.TrimSpace(string(importOutput)), strings.TrimSpace(string(moduleOutput)), nil
}

func runPackageProgram(importPath, moduleDir, body string) (string, error) {
	tempDir, err := os.MkdirTemp(moduleDir, "gosqlkit-generate-*")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	source := fmt.Sprintf(`package main

import (
	"fmt"
	"os"

	"github.com/webdeveloperben/gosqlkit/kit"
	_ "%s"
)

func main() {
%s
}
`, importPath, indent(body, "\t"))

	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(source), 0o600); err != nil {
		return "", err
	}

	rel, err := filepath.Rel(moduleDir, tempDir)
	if err != nil {
		return "", err
	}

	// #nosec G204 -- rel points at the generated temporary program under moduleDir.
	cmd := exec.Command("go", "run", "./"+filepath.ToSlash(rel))
	cmd.Dir = moduleDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go run generator: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

func dialect(value string) string {
	if value == "" {
		return "postgres"
	}
	return value
}

func indent(value, prefix string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

func commandError(context string, err error) error {
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return fmt.Errorf("%s: %w\n%s", context, err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return fmt.Errorf("%s: %w", context, err)
}

func writeOutput(path, content string) error {
	// #nosec G301 -- generated schema files should follow normal project-readable permissions.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// #nosec G306 -- generated schema files should follow normal project-readable permissions.
	return os.WriteFile(path, []byte(content), 0o644)
}

func checkOutput(path, content string) error {
	// #nosec G304 -- --out is intentionally user-selected and resolved relative to the project root.
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(current) != content {
		return fmt.Errorf("%s is out of date; run gosqlkit generate --out %s", path, path)
	}
	return nil
}
