package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
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
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("9")).
		Render("database schema drift detected")
	if _, err := fmt.Fprintln(w, title); err != nil {
		return err
	}
	if len(result.Differences) == 0 {
		_, err := fmt.Fprintf(w, "\ndatabase snapshot %s does not match desired snapshot %s\n", result.DatabaseSnapshotID, result.DesiredSnapshotID)
		return err
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, driftDifferenceTable(result.Differences))
	return err
}

func driftDifferenceTable(differences []app.DriftDifference) string {
	groups := map[string][]app.DriftDifference{}
	for _, diff := range differences {
		groups[diff.Op] = append(groups[diff.Op], diff)
	}
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("8"))).
		Headers("Status", "Kind", "Object", "Fields").
		StyleFunc(func(row, _ int) lipgloss.Style {
			style := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return style.Bold(true).Foreground(lipgloss.Color("12"))
			}
			return style
		})
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
		for _, item := range items {
			fields := ""
			if len(item.Fields) > 0 {
				fields = strings.Join(item.Fields, ", ")
			}
			t.Row(op, item.Object.Kind, item.Object.Key, fields)
		}
	}
	return t.String()
}
