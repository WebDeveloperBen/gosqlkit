package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/webdeveloperben/gosqlkit/kit"
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
	Stdout       io.Writer
	Package      string
	Dialect      string
	Out          string
	Root         string
	PreviousSnap string
	Check        bool
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
	if err := requireDialectCapability(dialect(opts.Dialect), kit.CapabilityRenderSQL); err != nil {
		return nil, err
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

	sql, importPaths, err := renderSQLWithConfig(config)
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
	if err := requireDialectCapability(dialect(opts.Dialect), kit.CapabilitySnapshotJSON); err != nil {
		return nil, err
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

	prevID, err := readPreviousSnapshotID(opts.PreviousSnap)
	if err != nil {
		return nil, err
	}

	rawJSON, err := runPackageProgram([]string{importPath}, kitImportPath, moduleDir, snapshotBody(dialect(opts.Dialect)))
	if err != nil {
		return nil, err
	}

	json, err := injectSnapshotIDs(rawJSON, prevID)
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

	prevID, err := readPreviousSnapshotID(opts.PreviousSnap)
	if err != nil {
		return nil, err
	}

	json, importPaths, err := renderSnapshotWithConfig(config, prevID)
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

func renderSQLWithConfig(config *Config) (sql string, importPaths []string, err error) {
	if err := requireDialectCapability(dialect(config.Dialect), kit.CapabilityRenderSQL); err != nil {
		return "", nil, err
	}

	importPaths, moduleDir, kitImportPath, err := resolveConfigProgram(config)
	if err != nil {
		return "", nil, err
	}

	d := dialect(config.Dialect)
	sql, err = runPackageProgram(importPaths, kitImportPath, moduleDir, fmt.Sprintf(`sql, err := kit.RenderSQL(%q)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(sql)`, d))
	if err != nil {
		return "", nil, err
	}
	return sql, importPaths, nil
}

func renderSnapshotWithConfig(config *Config, previousSnapshotID string) (snapshot string, importPaths []string, err error) {
	if err := requireDialectCapability(dialect(config.Dialect), kit.CapabilitySnapshotJSON); err != nil {
		return "", nil, err
	}

	importPaths, moduleDir, kitImportPath, err := resolveConfigProgram(config)
	if err != nil {
		return "", nil, err
	}

	rawJSON, err := runPackageProgram(importPaths, kitImportPath, moduleDir, snapshotBody(dialect(config.Dialect)))
	if err != nil {
		return "", nil, err
	}
	snapshot, err = injectSnapshotIDs(rawJSON, previousSnapshotID)
	if err != nil {
		return "", nil, err
	}
	return snapshot, importPaths, nil
}

func resolveConfigProgram(config *Config) (importPaths []string, moduleDir, kitImportPath string, err error) {
	root, err := ResolveRoot(config.RootDir())
	if err != nil {
		return nil, "", "", err
	}

	importPaths, moduleDir, err = resolvePackages(root, config.SchemaPaths())
	if err != nil {
		return nil, "", "", err
	}

	kitImportPath, err = resolveKitImportPath(root)
	if err != nil {
		return nil, "", "", err
	}
	return importPaths, moduleDir, kitImportPath, nil
}

func snapshotBody(dialect string) string {
	return fmt.Sprintf(`data, err := kit.SnapshotJSON(%q)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}`, dialect)
}

func readPreviousSnapshotID(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	// #nosec G304 -- previous snapshot path is explicitly user-provided via CLI flag or config; validated to be a file path.
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read previous snapshot %q: %w", path, err)
	}
	var raw struct {
		SnapshotID string `json:"snapshotId"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("parse previous snapshot %q: %w", path, err)
	}
	if raw.SnapshotID == "" {
		return "", fmt.Errorf("previous snapshot %q has no snapshotId", path)
	}
	return raw.SnapshotID, nil
}

func snapshotIDFromJSON(data string) (string, error) {
	var raw struct {
		SnapshotID string `json:"snapshotId"`
	}
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return "", fmt.Errorf("parse snapshot JSON: %w", err)
	}
	if raw.SnapshotID == "" {
		return "", errors.New("snapshot JSON has no snapshotId")
	}
	return raw.SnapshotID, nil
}

func injectSnapshotIDs(rawJSON, previousSnapshotID string) (string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return "", fmt.Errorf("parse snapshot JSON: %w", err)
	}

	delete(raw, "snapshotId")
	delete(raw, "previousSnapshotId")

	if previousSnapshotID != "" {
		prevData, err := json.Marshal(previousSnapshotID)
		if err != nil {
			return "", err
		}
		raw["previousSnapshotId"] = prevData
	}

	content, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(content)
	snapshotID := hashToString(hash[:])

	idData, err := json.Marshal(snapshotID)
	if err != nil {
		return "", err
	}
	raw["snapshotId"] = idData

	final, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return "", err
	}
	return string(append(final, '\n')), nil
}

func hashToString(b []byte) string {
	const hex = "0123456789abcdef"
	buf := make([]byte, len(b)*2)
	for i, v := range b {
		buf[i*2] = hex[v>>4]
		buf[i*2+1] = hex[v&0x0f]
	}
	return string(buf)
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

	// #nosec G204 -- tempDir is a secure OS temporary directory; main.go is generated code, not user input.
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
	// #nosec G301 -- generated schema directory creation uses user-resolved path with standard permissions.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// #nosec G306 -- generated schema files use standard read-write permissions (0644) for project files.
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	return nil
}

func checkOutput(path, content string) error {
	// #nosec G304 -- --out path is explicitly user-selected via CLI or config; resolved relative to project root with validation.
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
