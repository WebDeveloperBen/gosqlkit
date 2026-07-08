package cli

import (
	"fmt"
	"io"
	"strconv"

	"github.com/webdeveloperben/gosqlkit/internal/app"
)

type CICmd struct {
	Database CIDatabaseCmd `cmd:"" help:"Check a database against the generated schema for CI."`
	Schema   CISchemaCmd   `cmd:"" help:"Check committed generated schema, snapshot, and migrations for CI."`
}

type CISchemaCmd struct {
	Config              string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir                 string `help:"Migration directory to validate. Overrides config migrations.dir." type:"path"`
	SandboxURL          string `help:"PostgreSQL URL for replaying migrations into a disposable sandbox database."`
	SandboxURLEnv       string `name:"sandbox-url-env" help:"Environment variable containing the sandbox PostgreSQL URL."`
	SandboxTokenCommand string `name:"sandbox-token-command" help:"Command that prints a sandbox database auth token to stdout. The token is used as the PostgreSQL password."`
	JSON                bool   `help:"Emit machine-readable JSON instead of human-readable text."`
	Quiet               bool   `help:"Suppress human-readable success output."`
}

func (c *CISchemaCmd) Run(g *GlobalFlags) error {
	config, err := loadCLIConfig(g.Root, c.Config)
	if err != nil {
		return Exit(1, err)
	}

	result, err := app.CISchemaWithConfig(config, app.CISchemaOptions{
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
	if !printHuman(c.JSON, c.Quiet) {
		return nil
	}
	return printCISchema(g.stdout(), result)
}

type CIDatabaseCmd struct {
	Config       string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	URL          string `help:"Database URL to inspect. Defaults to DATABASE_URL when omitted."`
	URLEnv       string `name:"url-env" help:"Environment variable containing the database URL."`
	TokenCommand string `name:"token-command" help:"Command that prints a database auth token to stdout. The token is used as the database password."`
	JSON         bool   `help:"Emit machine-readable JSON instead of human-readable text."`
	Quiet        bool   `help:"Suppress human-readable success output."`
}

func (c *CIDatabaseCmd) Run(g *GlobalFlags) error {
	config, err := loadCLIConfig(g.Root, c.Config)
	if err != nil {
		return Exit(1, err)
	}

	result, err := app.CIDatabaseWithConfig(config, app.CIDatabaseOptions{
		URL:          c.URL,
		URLEnv:       c.URLEnv,
		TokenCommand: c.TokenCommand,
	})
	if c.JSON && result != nil {
		if printErr := printJSON(g.stdout(), result); printErr != nil {
			return printErr
		}
	}
	if err != nil {
		if result != nil && result.Drift != nil && result.Drift.Drift && printHuman(c.JSON, c.Quiet) {
			if printErr := printDrift(g.stdout(), result.Drift); printErr != nil {
				return printErr
			}
		}
		return Exit(1, err)
	}
	if !printHuman(c.JSON, c.Quiet) {
		return nil
	}
	return printCIDatabase(g.stdout(), result)
}

func loadCLIConfig(rootFlag, configPath string) (*app.Config, error) {
	root, err := app.ResolveRoot(rootFlag)
	if err != nil {
		return nil, err
	}
	if configPath == "" {
		if configPath, err = discoverConfigPath(root); err != nil {
			return nil, err
		}
	}
	return app.LoadConfig(configPath)
}

func printCISchema(w io.Writer, result *app.CISchemaResult) error {
	if result == nil {
		return nil
	}
	t := styledTable("Check", "Status", "Detail")
	if result.Generate != nil {
		t.Row("generated SQL", "ok", result.Generate.Out)
	}
	if result.Snapshot != nil {
		t.Row("snapshot JSON", "ok", result.Snapshot.Out)
	}
	if result.Migration != nil {
		t.Row("migrations", "ok", strconv.Itoa(result.Migration.Count))
		if result.Migration.Sandbox != nil {
			t.Row("sandbox replay", "ok", result.Migration.Sandbox.LastFile)
		}
	}
	_, err := fmt.Fprintln(w, t.String())
	return err
}

func printCIDatabase(w io.Writer, result *app.CIDatabaseResult) error {
	if result == nil || result.Drift == nil {
		return nil
	}
	t := styledTable("Check", "Status", "Detail")
	t.Row("database drift", "ok", result.Drift.DatabaseSnapshotID)
	_, err := fmt.Fprintln(w, t.String())
	return err
}
