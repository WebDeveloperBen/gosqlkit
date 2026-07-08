package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/app"
)

type MigrateCmd struct {
	Check  MigrateCheckCmd  `cmd:"" help:"Validate migration files."`
	Create MigrateCreateCmd `cmd:"" help:"Create a migration file."`
	Plan   MigratePlanCmd   `cmd:"" help:"Print the structured migration plan without writing files."`
}

type MigrateCreateCmd struct {
	Config           string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir              string `help:"Write migration files to this directory. Overrides config migrations.dir." type:"path"`
	Runner           string `help:"Migration runner file format. Currently only goose is supported."`
	Name             string `arg:"" help:"Migration name."`
	Empty            bool   `help:"Create an empty migration for manual SQL."`
	NoDown           bool   `help:"Omit the down migration section."`
	AllowDestructive bool   `help:"Allow the migration to contain destructive changes (drops, disables, comment removals)."`
}

func (c *MigrateCreateCmd) Run(g *GlobalFlags) error {
	root, err := app.ResolveRoot(g.Root)
	if err != nil {
		return Exit(1, err)
	}

	configPath := c.Config
	if configPath == "" {
		if configPath, err = discoverConfigPath(root); err != nil {
			return Exit(1, err)
		}
	}

	config, err := app.LoadConfig(configPath)
	if err != nil {
		return Exit(1, err)
	}

	if _, err := app.MigrateCreateWithConfig(config, app.MigrateCreateOptions{
		Name:             c.Name,
		Dir:              c.Dir,
		Runner:           c.Runner,
		Empty:            c.Empty,
		NoDown:           c.NoDown,
		AllowDestructive: c.AllowDestructive,
	}); err != nil {
		return Exit(1, err)
	}
	return nil
}

type MigrateCheckCmd struct {
	Config     string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir        string `help:"Migration directory to validate. Overrides config migrations.dir." type:"path"`
	SandboxURL string `help:"PostgreSQL URL for replaying migrations into a disposable sandbox database."`
}

func (c *MigrateCheckCmd) Run(g *GlobalFlags) error {
	root, err := app.ResolveRoot(g.Root)
	if err != nil {
		return Exit(1, err)
	}

	configPath := c.Config
	if configPath == "" {
		if configPath, err = discoverConfigPath(root); err != nil {
			return Exit(1, err)
		}
	}

	config, err := app.LoadConfig(configPath)
	if err != nil {
		return Exit(1, err)
	}

	if _, err := app.MigrateCheckWithConfig(config, app.MigrateCheckOptions{
		Dir:        c.Dir,
		SandboxURL: c.SandboxURL,
	}); err != nil {
		return Exit(1, err)
	}
	return nil
}

type MigratePlanCmd struct {
	Config   string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir      string `help:"Migration directory containing the previous migration. Overrides config migrations.dir." type:"path"`
	Runner   string `help:"Migration runner file format. Currently only goose is supported."`
	Snapshot string `help:"Snapshot file to diff against. Defaults to the config snapshot output."`
	JSON     bool   `help:"Emit machine-readable JSON instead of human-readable text."`
}

func (c *MigratePlanCmd) Run(g *GlobalFlags) error {
	root, err := app.ResolveRoot(g.Root)
	if err != nil {
		return Exit(1, err)
	}

	configPath := c.Config
	if configPath == "" {
		if configPath, err = discoverConfigPath(root); err != nil {
			return Exit(1, err)
		}
	}

	config, err := app.LoadConfig(configPath)
	if err != nil {
		return Exit(1, err)
	}

	result, err := app.MigratePlanWithConfig(config, app.MigratePlanOptions{
		Dir:      c.Dir,
		Runner:   c.Runner,
		Snapshot: c.Snapshot,
	})
	if err != nil {
		return Exit(1, err)
	}

	if c.JSON {
		return printJSON(os.Stdout, result)
	}
	return printPlan(os.Stdout, result)
}

func printJSON(w io.Writer, result *app.MigratePlanResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return Exit(1, fmt.Errorf("marshal plan: %w", err))
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func printPlan(w io.Writer, result *app.MigratePlanResult) error {
	if result == nil {
		return nil
	}
	if len(result.Changes) == 0 {
		_, err := fmt.Fprintln(w, "no changes")
		return err
	}
	if _, err := fmt.Fprintf(w, "dialect:        %s\n", result.Dialect); err != nil {
		return err
	}
	if result.FromSnapshotID != "" {
		if _, err := fmt.Fprintf(w, "fromSnapshot:   %s\n", result.FromSnapshotID); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "toSnapshot:     %s\n", result.ToSnapshotID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "changes:        %d\n", len(result.Changes)); err != nil {
		return err
	}
	if len(result.Destructive) > 0 {
		if _, err := fmt.Fprintf(w, "destructive:    %d (pass --allow-destructive to write)\n", len(result.Destructive)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	for _, change := range result.Changes {
		risks := ""
		if len(change.Risks) > 0 {
			risks = "  risks=" + strings.Join(change.Risks, ",")
		}
		if _, err := fmt.Fprintf(w, "  %s %s%s\n", change.Op, change.Object.Key, risks); err != nil {
			return err
		}
	}
	return nil
}
