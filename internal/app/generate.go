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

const pgImportPath = "github.com/webdeveloperben/pgkit/pg"

type GenerateOptions struct {
	Stdout  io.Writer
	Package string
	Out     string
	Root    string
	Check   bool
}

type GenerateResult struct {
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

	sql, err := renderPackage(importPath, moduleDir)
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

func renderPackage(importPath, moduleDir string) (string, error) {
	tempDir, err := os.MkdirTemp(moduleDir, "pgkit-generate-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	source := fmt.Sprintf(`package main

import (
	"fmt"
	"os"

	"%s"
	_ "%s"
)

func main() {
	sql, err := pg.Render()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(sql)
}
`, pgImportPath, importPath)

	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(source), 0o600); err != nil {
		return "", err
	}

	rel, err := filepath.Rel(moduleDir, tempDir)
	if err != nil {
		return "", err
	}

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

func commandError(context string, err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("%s: %w\n%s", context, err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return fmt.Errorf("%s: %w", context, err)
}

func writeOutput(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func checkOutput(path, content string) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(current) != content {
		return fmt.Errorf("%s is out of date; run pgkit generate --out %s", path, path)
	}
	return nil
}
