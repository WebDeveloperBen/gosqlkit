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
	"text/template"
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
	Out      string   `json:"out,omitempty"`
	Package  string   `json:"package,omitempty"`
	Packages []string `json:"packages,omitempty"`
	Checked  bool     `json:"checked"`
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
	Out      string   `json:"out,omitempty"`
	Package  string   `json:"package,omitempty"`
	Packages []string `json:"packages,omitempty"`
	Checked  bool     `json:"checked"`
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

	kitImportPath, err := resolveKitImportPath(root)
	if err != nil {
		return nil, err
	}

	sql, err := runPackageProgram([]string{importPath}, kitImportPath, moduleDir, fmt.Sprintf(`sql, err := kit.RenderSQL(%q)
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

func GenerateWithConfig(config *Config, opts GenerateOptions) (*GenerateResult, error) {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}

	root, err := ResolveRoot(config.RootDir())
	if err != nil {
		return nil, err
	}

	importPaths, moduleDir, err := resolvePackages(root, config.SchemaPaths())
	if err != nil {
		return nil, err
	}

	kitImportPath, err := resolveKitImportPath(root)
	if err != nil {
		return nil, err
	}

	d := dialect(config.Dialect)
	sql, err := runPackageProgram(importPaths, kitImportPath, moduleDir, fmt.Sprintf(`sql, err := kit.RenderSQL(%q)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(sql)`, d))
	if err != nil {
		return nil, err
	}

	out := config.SQLPath()
	if opts.Out != "" {
		out = config.ResolvePath(opts.Out)
	}
	if opts.Check {
		if out == "" {
			return nil, errors.New("generate --check requires --out or config out.sql")
		}
		if err := checkOutput(out, sql); err != nil {
			return nil, err
		}
		return &GenerateResult{Checked: true, Out: out, Packages: importPaths}, nil
	}
	if out != "" {
		if err := writeOutput(out, sql); err != nil {
			return nil, err
		}
		return &GenerateResult{Out: out, Packages: importPaths}, nil
	}

	if _, err := io.WriteString(opts.Stdout, sql); err != nil {
		return nil, err
	}
	return &GenerateResult{Packages: importPaths}, nil
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

	kitImportPath, err := resolveKitImportPath(root)
	if err != nil {
		return nil, err
	}

	json, err := runPackageProgram([]string{importPath}, kitImportPath, moduleDir, fmt.Sprintf(`data, err := kit.SnapshotJSON(%q)
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

func SnapshotWithConfig(config *Config, opts SnapshotOptions) (*SnapshotResult, error) {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}

	root, err := ResolveRoot(config.RootDir())
	if err != nil {
		return nil, err
	}

	importPaths, moduleDir, err := resolvePackages(root, config.SchemaPaths())
	if err != nil {
		return nil, err
	}

	kitImportPath, err := resolveKitImportPath(root)
	if err != nil {
		return nil, err
	}

	d := dialect(config.Dialect)
	json, err := runPackageProgram(importPaths, kitImportPath, moduleDir, fmt.Sprintf(`data, err := kit.SnapshotJSON(%q)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}`, d))
	if err != nil {
		return nil, err
	}

	out := config.SnapshotPath()
	if opts.Out != "" {
		out = config.ResolvePath(opts.Out)
	}
	if opts.Check {
		if out == "" {
			return nil, errors.New("snapshot --check requires --out or config out.snapshot")
		}
		if err := checkOutput(out, json); err != nil {
			return nil, err
		}
		return &SnapshotResult{Checked: true, Out: out, Packages: importPaths}, nil
	}
	if out != "" {
		if err := writeOutput(out, json); err != nil {
			return nil, err
		}
		return &SnapshotResult{Out: out, Packages: importPaths}, nil
	}

	if _, err := io.WriteString(opts.Stdout, json); err != nil {
		return nil, err
	}
	return &SnapshotResult{Packages: importPaths}, nil
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

	moduleDir, err = resolveModuleDir(root)
	if err != nil {
		return "", "", err
	}

	return strings.TrimSpace(string(importOutput)), moduleDir, nil
}

func resolvePackages(root string, paths []string) (importPaths []string, moduleDir string, err error) {
	moduleDir, err = resolveModuleDir(root)
	if err != nil {
		return nil, "", err
	}

	seen := map[string]struct{}{}
	for _, path := range paths {
		// #nosec G204 -- path is a user-supplied Go package path from the config file.
		listCmd := exec.Command("go", "list", "-f", "{{.ImportPath}}", path)
		listCmd.Dir = root
		output, err := listCmd.Output()
		if err != nil {
			return nil, "", commandError(fmt.Sprintf("go list package %q", path), err)
		}
		importPath := strings.TrimSpace(string(output))
		if _, ok := seen[importPath]; ok {
			continue
		}
		seen[importPath] = struct{}{}
		importPaths = append(importPaths, importPath)
	}

	return importPaths, moduleDir, nil
}

func resolveModule(root string) (modulePath string, moduleDir string, err error) {
	// #nosec G204 -- go list is a fixed command, not user input.
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}\n{{.Dir}}")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return "", "", commandError("go list module", err)
	}
	lines := strings.SplitN(strings.TrimSpace(string(output)), "\n", 2)
	if len(lines) < 2 {
		return "", "", fmt.Errorf("unexpected go list -m output: %q", string(output))
	}
	return lines[0], lines[1], nil
}

func resolveModuleDir(root string) (string, error) {
	// #nosec G204 -- go list is a fixed command, not user input.
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return "", commandError("go list module", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func resolveKitImportPath(root string) (string, error) {
	modulePath, _, err := resolveModule(root)
	if err != nil {
		return "", err
	}
	return modulePath + "/kit", nil
}

func runPackageProgram(importPaths []string, kitImportPath, moduleDir, body string) (string, error) {
	tempDir, err := os.MkdirTemp("", "gosqlkit-generate-*")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	source, err := renderGeneratorProgram(importPaths, kitImportPath, body)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(source), 0o600); err != nil {
		return "", err
	}

	// #nosec G204 -- tempDir points at the generated temporary program in the OS temp dir.
	cmd := exec.Command("go", "run", filepath.Join(tempDir, "main.go"))
	cmd.Dir = moduleDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go run generator: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

func renderGeneratorProgram(importPaths []string, kitImportPath, body string) (string, error) {
	tmpl, err := template.New("generator").Parse(generatorTemplate)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	if err := tmpl.Execute(&b, struct {
		KitImportPath string
		Body          string
		ImportPaths   []string
	}{
		KitImportPath: kitImportPath,
		ImportPaths:   importPaths,
		Body:          indent(body, "\t"),
	}); err != nil {
		return "", err
	}

	return b.String(), nil
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

const generatorTemplate = `package main

import (
	"fmt"
	"os"

	"{{.KitImportPath}}"
{{- range .ImportPaths}}
	_ {{printf "%q" .}}
{{- end}}
)

func main() {
{{.Body}}
}
`
