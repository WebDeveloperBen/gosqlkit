package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/webdeveloperben/gosqlkit/internal/app"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
)

type MigrateCmd struct {
	Apply  MigrateApplyCmd  `cmd:"" help:"Apply pending migrations to a PostgreSQL database."`
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
	Interactive      bool   `help:"Prompt for ambiguous migration choices when stdin/stdout are TTYs."`
	NoInteractive    bool   `name:"no-interactive" help:"Disable prompts and fail closed on ambiguous migrations."`
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
	if c.Interactive && c.NoInteractive {
		return Exit(1, errors.New("--interactive and --no-interactive cannot be used together"))
	}

	interaction := app.InteractionAuto
	if c.Interactive {
		interaction = app.InteractionAlways
	}
	if c.NoInteractive {
		interaction = app.InteractionNever
	}
	if _, err := app.MigrateCreateWithConfig(config, app.MigrateCreateOptions{
		Name:             c.Name,
		Dir:              c.Dir,
		Runner:           c.Runner,
		Empty:            c.Empty,
		NoDown:           c.NoDown,
		AllowDestructive: c.AllowDestructive,
		Interaction:      interaction,
	}); err != nil {
		return Exit(1, err)
	}
	return nil
}

type MigrateCheckCmd struct {
	Config              string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir                 string `help:"Migration directory to validate. Overrides config migrations.dir." type:"path"`
	SandboxURL          string `help:"PostgreSQL URL for replaying migrations into a disposable sandbox database."`
	SandboxURLEnv       string `name:"sandbox-url-env" help:"Environment variable containing the sandbox PostgreSQL URL."`
	SandboxTokenCommand string `name:"sandbox-token-command" help:"Command that prints a sandbox database auth token to stdout. The token is used as the PostgreSQL password."`
	JSON                bool   `help:"Emit machine-readable JSON instead of human-readable text."`
	Quiet               bool   `help:"Suppress human-readable success output."`
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

	result, err := app.MigrateCheckWithConfig(config, app.MigrateCheckOptions{
		Dir:                 c.Dir,
		SandboxURL:          c.SandboxURL,
		SandboxURLEnv:       c.SandboxURLEnv,
		SandboxTokenCommand: c.SandboxTokenCommand,
	})
	if c.JSON && result != nil {
		if printErr := printJSON(g.stdout(), result); printErr != nil {
			return printErr
		}
	}
	if err != nil {
		return Exit(1, err)
	}
	return nil
}

type MigrateApplyCmd struct {
	Config       string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir          string `help:"Migration directory to apply. Overrides config migrations.dir." type:"path"`
	Runner       string `help:"Migration runner file format. Currently only goose is supported."`
	URL          string `help:"PostgreSQL URL for the target database. Defaults to DATABASE_URL when omitted."`
	URLEnv       string `name:"url-env" help:"Environment variable containing the PostgreSQL URL."`
	TokenCommand string `name:"token-command" help:"Command that prints a database auth token to stdout. The token is used as the PostgreSQL password."`
	JSON         bool   `help:"Emit machine-readable JSON instead of human-readable text."`
	Quiet        bool   `help:"Suppress human-readable success output."`
}

func (c *MigrateApplyCmd) Run(g *GlobalFlags) error {
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

	result, err := app.MigrateApplyWithConfig(config, app.MigrateApplyOptions{
		Dir:          c.Dir,
		Runner:       c.Runner,
		URL:          c.URL,
		URLEnv:       c.URLEnv,
		TokenCommand: c.TokenCommand,
	})
	if err != nil {
		return Exit(1, err)
	}

	if c.JSON {
		return printJSON(g.stdout(), result)
	}
	if !printHuman(c.JSON, c.Quiet) {
		return nil
	}
	return printApply(g.stdout(), result)
}

type MigratePlanCmd struct {
	Config   string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir      string `help:"Migration directory containing the previous migration. Overrides config migrations.dir." type:"path"`
	Runner   string `help:"Migration runner file format. Currently only goose is supported."`
	Snapshot string `help:"Snapshot file to diff against. Defaults to the config snapshot output."`
	JSON     bool   `help:"Emit machine-readable JSON instead of human-readable text."`
	Quiet    bool   `help:"Suppress human-readable success output."`
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
		return printJSON(g.stdout(), result)
	}
	if !printHuman(c.JSON, c.Quiet) {
		return nil
	}
	return printPlan(g.stdout(), result)
}

func printHuman(jsonOutput, quiet bool) bool {
	return !jsonOutput && !quiet
}

func printJSON(w io.Writer, result any) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return Exit(1, fmt.Errorf("marshal JSON: %w", err))
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func printApply(w io.Writer, result *app.MigrateApplyResult) error {
	if result == nil {
		return nil
	}
	if _, err := fmt.Fprintf(w, "migrations: %d\n", result.Count); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "applied:    %d\n", result.Applied); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "skipped:    %d\n", result.Skipped); err != nil {
		return err
	}
	if result.LastFile != "" {
		if _, err := fmt.Fprintf(w, "lastFile:   %s\n", result.LastFile); err != nil {
			return err
		}
	}
	return nil
}

func printPlan(w io.Writer, result *app.MigratePlanResult) error {
	if result == nil {
		return nil
	}
	if len(result.Changes) == 0 {
		_, err := fmt.Fprintln(w, "no changes")
		return err
	}
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Render("migration plan")
	if _, err := fmt.Fprintln(w, title); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, planSummaryTable(result)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, planChangeTable(result.Changes))
	return err
}

func planSummaryTable(result *app.MigratePlanResult) string {
	t := styledTable("Field", "Value")
	t.Row("Dialect", result.Dialect)
	if result.FromSnapshotID != "" {
		t.Row("From snapshot", result.FromSnapshotID)
	}
	t.Row("To snapshot", result.ToSnapshotID)
	t.Row("Changes", strconv.Itoa(len(result.Changes)))
	t.Row("Up statements", strconv.Itoa(len(result.Statements)))
	if len(result.Destructive) > 0 {
		t.Row("Destructive", fmt.Sprintf("%d (pass --allow-destructive to write)", len(result.Destructive)))
	}
	return t.String()
}

func planChangeTable(changes []migrate.Change) string {
	t := styledTable("Op", "Kind", "Object", "Risks", "Summary")
	for _, change := range changes {
		risks := strings.Join(change.Risks, ", ")
		t.Row(change.Op, change.Object.Kind, change.Object.Key, risks, change.Summary)
	}
	return t.String()
}

func styledTable(headers ...string) *table.Table {
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("8"))).
		Headers(headers...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			style := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return style.Bold(true).Foreground(lipgloss.Color("12"))
			}
			return style
		})
}
