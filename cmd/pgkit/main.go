package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const pgImportPath = "github.com/webdeveloperben/pgkit/pg"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return errors.New("usage: pgkit generate <schema-package>")
	}

	switch args[0] {
	case "generate":
		return runGenerate(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runGenerate(args []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: pgkit generate <schema-package>")
	}

	pkg := flags.Arg(0)
	importPath, moduleDir, err := resolvePackage(pkg)
	if err != nil {
		return err
	}

	sql, err := generateFromPackage(importPath, moduleDir)
	if err != nil {
		return err
	}
	_, err = stdout.Write([]byte(sql))
	return err
}

func resolvePackage(pkg string) (importPath string, moduleDir string, err error) {
	listCmd := exec.Command("go", "list", "-f", "{{.ImportPath}}", pkg)
	importOutput, err := listCmd.Output()
	if err != nil {
		return "", "", commandError("go list package", err)
	}

	moduleCmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
	moduleOutput, err := moduleCmd.Output()
	if err != nil {
		return "", "", commandError("go list module", err)
	}

	return strings.TrimSpace(string(importOutput)), strings.TrimSpace(string(moduleOutput)), nil
}

func generateFromPackage(importPath, moduleDir string) (string, error) {
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
