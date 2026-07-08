package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/app"
)

type DriftCmd struct {
	Check DriftCheckCmd `cmd:"" help:"Check a database against the generated schema snapshot."`
}

type DriftCheckCmd struct {
	Config       string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	URL          string `help:"Database URL to inspect. Defaults to DATABASE_URL when omitted."`
	URLEnv       string `name:"url-env" help:"Environment variable containing the database URL."`
	TokenCommand string `name:"token-command" help:"Command that prints a database auth token to stdout. The token is used as the database password."`
	JSON         bool   `help:"Emit machine-readable JSON instead of human-readable text."`
	Quiet        bool   `help:"Suppress human-readable success output."`
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
		if result != nil && result.Drift && printHuman(c.JSON, c.Quiet) {
			if printErr := printDrift(g.stdout(), result); printErr != nil {
				return printErr
			}
		}
		return Exit(1, err)
	}
	return nil
}

func printDrift(w io.Writer, result *app.DriftCheckResult) error {
	if result == nil || !result.Drift {
		return nil
	}
	if _, err := fmt.Fprintln(w, "database schema drift detected"); err != nil {
		return err
	}
	if len(result.Differences) == 0 {
		_, err := fmt.Fprintf(w, "\ndatabase snapshot %s does not match desired snapshot %s\n", result.DatabaseSnapshotID, result.DesiredSnapshotID)
		return err
	}

	groups := map[string][]app.DriftDifference{}
	for _, diff := range result.Differences {
		groups[diff.Op] = append(groups[diff.Op], diff)
	}
	for _, op := range []string{"missing", "extra", "changed"} {
		items := groups[op]
		if len(items) == 0 {
			continue
		}
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Object.Kind == items[j].Object.Kind {
				return items[i].Object.Key < items[j].Object.Key
			}
			return items[i].Object.Kind < items[j].Object.Kind
		})
		if _, err := fmt.Fprintf(w, "\n%s:\n", op); err != nil {
			return err
		}
		for _, item := range items {
			line := fmt.Sprintf("  %s %s", item.Object.Kind, item.Object.Key)
			if len(item.Fields) > 0 {
				line += " (" + strings.Join(item.Fields, ", ") + ")"
			}
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
	}
	return nil
}
