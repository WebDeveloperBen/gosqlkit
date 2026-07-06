package cli

import (
	"fmt"

	"github.com/webdeveloperben/pgkit/internal/version"
)

type VersionCmd struct{}

func (c *VersionCmd) Run(g *GlobalFlags) error {
	_, err := fmt.Fprintln(g.stdout(), version.String())
	return err
}
