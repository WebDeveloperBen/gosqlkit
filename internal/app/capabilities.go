package app

import (
	"github.com/webdeveloperben/gosqlkit/kit"

	_ "github.com/webdeveloperben/gosqlkit/internal/providers"
)

func requireDialectCapability(dialect string, capability kit.Capability) error {
	_, err := kit.RequireCapability(dialect, capability)
	return err
}
