package cli

import "github.com/webdeveloperben/gosqlkit/internal/app"

type GenerateCmd struct {
	Config  string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Out     string `help:"Write generated SQL to path instead of stdout. Overrides config out.sql." type:"path"`
	Dialect string `help:"Database dialect to generate for. Overrides config dialect."`
	Package string `arg:"" name:"schema-package" optional:"" help:"Go package containing gosqlkit schema definitions. If omitted, reads from gosqlkit.yaml."`
	Check   bool   `help:"Fail if --out does not already match generated SQL."`
}

func (c *GenerateCmd) Run(g *GlobalFlags) error {
	if c.Package != "" {
		if _, err := app.Generate(app.GenerateOptions{
			Package: c.Package,
			Dialect: c.Dialect,
			Out:     c.Out,
			Check:   c.Check,
			Root:    g.Root,
			Stdout:  g.stdout(),
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

	if _, err := app.GenerateWithConfig(config, app.GenerateOptions{
		Out:    c.Out,
		Check:  c.Check,
		Stdout: g.stdout(),
	}); err != nil {
		return Exit(1, err)
	}
	return nil
}
