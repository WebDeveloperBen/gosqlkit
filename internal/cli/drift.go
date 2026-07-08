package cli

import (
	"github.com/webdeveloperben/gosqlkit/internal/app"
)

type DriftCmd struct {
	Check DriftCheckCmd `cmd:"" help:"Check a PostgreSQL database against the generated schema snapshot."`
}

type DriftCheckCmd struct {
	Config string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	URL    string `help:"PostgreSQL URL for the database to inspect. Defaults to DATABASE_URL when omitted."`
	URLEnv string `name:"url-env" help:"Environment variable containing the PostgreSQL URL."`
	JSON   bool   `help:"Emit machine-readable JSON instead of human-readable text."`
}

func (c *DriftCheckCmd) Run(g *GlobalFlags) error {
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

	result, err := app.DriftCheckWithConfig(config, app.DriftCheckOptions{
		URL:    c.URL,
		URLEnv: c.URLEnv,
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
