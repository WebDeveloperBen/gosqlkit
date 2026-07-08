package cli

import "github.com/webdeveloperben/gosqlkit/internal/app"

type InspectCmd struct {
	Config          string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	URL             string `help:"Database URL to inspect. Defaults to DATABASE_URL when omitted."`
	URLEnv          string `name:"url-env" help:"Environment variable containing the database URL."`
	TokenCommand    string `name:"token-command" help:"Command that prints a database auth token to stdout. The token is used as the database password."`
	Out             string `help:"Write inspected snapshot JSON to path instead of stdout." type:"path"`
	JSON            bool   `help:"Print inspected snapshot JSON to stdout even when --out is set. Snapshot JSON is printed by default when --out is omitted."`
	DriftProjection bool   `name:"drift-projection" help:"Emit the normalised database shape used by drift checks."`
}

func (c *InspectCmd) Run(g *GlobalFlags) error {
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

	if _, err := app.InspectWithConfig(config, app.InspectOptions{
		URL:             c.URL,
		URLEnv:          c.URLEnv,
		TokenCommand:    c.TokenCommand,
		Out:             c.Out,
		Stdout:          g.stdout(),
		JSON:            c.JSON,
		DriftProjection: c.DriftProjection,
	}); err != nil {
		return Exit(1, err)
	}
	return nil
}
