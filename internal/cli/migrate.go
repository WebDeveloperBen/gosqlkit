package cli

import "github.com/webdeveloperben/gosqlkit/internal/app"

type MigrateCmd struct {
	Check  MigrateCheckCmd  `cmd:"" help:"Validate migration files."`
	Create MigrateCreateCmd `cmd:"" help:"Create a migration file."`
}

type MigrateCreateCmd struct {
	Config string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir    string `help:"Write migration files to this directory. Overrides config migrations.dir." type:"path"`
	Runner string `help:"Migration runner file format. Currently only goose is supported."`
	Name   string `arg:"" help:"Migration name."`
	Empty  bool   `help:"Create an empty migration for manual SQL."`
	NoDown bool   `help:"Omit the down migration section."`
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
		Name:   c.Name,
		Dir:    c.Dir,
		Runner: c.Runner,
		Empty:  c.Empty,
		NoDown: c.NoDown,
	}); err != nil {
		return Exit(1, err)
	}
	return nil
}

type MigrateCheckCmd struct {
	Config string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir    string `help:"Migration directory to validate. Overrides config migrations.dir." type:"path"`
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
		Dir: c.Dir,
	}); err != nil {
		return Exit(1, err)
	}
	return nil
}
