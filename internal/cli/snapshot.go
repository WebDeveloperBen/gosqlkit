package cli

import "github.com/webdeveloperben/gosqlkit/internal/app"

type SnapshotCmd struct {
	Out     string `help:"Write generated snapshot JSON to path instead of stdout." type:"path"`
	Dialect string `help:"Database dialect to snapshot." default:"postgres"`
	Package string `arg:"" name:"schema-package" help:"Go package containing gosqlkit schema definitions."`
	Check   bool   `help:"Fail if --out does not already match generated snapshot JSON."`
}

func (c *SnapshotCmd) Run(g *GlobalFlags) error {
	if _, err := app.Snapshot(app.SnapshotOptions{
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
