package cli

import "github.com/webdeveloperben/gosqlkit/internal/app"

type SnapshotCmd struct {
	Config  string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Out     string `help:"Write generated snapshot JSON to path instead of stdout. Overrides config out.snapshot." type:"path"`
	Dialect string `help:"Database dialect to snapshot. Overrides config dialect."`
	Prev    string `help:"Path to previous snapshot JSON. Sets previousSnapshotId in the output." type:"path"`
	Package string `arg:"" name:"schema-package" optional:"" help:"Go package containing gosqlkit schema definitions. If omitted, reads from gosqlkit.yaml."`
	Check   bool   `help:"Fail if --out does not already match generated snapshot JSON."`
}

func (c *SnapshotCmd) Run(g *GlobalFlags) error {
	if c.Package != "" {
		if _, err := app.Snapshot(app.SnapshotOptions{
			Package:      c.Package,
			Dialect:      c.Dialect,
			Out:          c.Out,
			Check:        c.Check,
			PreviousSnap: c.Prev,
			Root:         g.Root,
			Stdout:       g.stdout(),
		}); err != nil {
			return Exit(1, err)
		}
		return nil
	}

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

	if c.Dialect != "" {
		config.Dialect = c.Dialect
	}

	if _, err := app.SnapshotWithConfig(config, app.SnapshotOptions{
		Out:          c.Out,
		Check:        c.Check,
		PreviousSnap: c.Prev,
		Stdout:       g.stdout(),
	}); err != nil {
		return Exit(1, err)
	}
	return nil
}
