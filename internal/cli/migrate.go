package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/webdeveloperben/gosqlkit/internal/app"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
)

type MigrateCmd struct {
	Apply  MigrateApplyCmd  `cmd:"" help:"Apply pending migrations to a PostgreSQL database."`
	Check  MigrateCheckCmd  `cmd:"" help:"Validate migration files."`
	Create MigrateCreateCmd `cmd:"" help:"Create a migration file."`
	Plan   MigratePlanCmd   `cmd:"" help:"Print the structured migration plan without writing files."`
}

type MigrateCreateCmd struct {
	Config           string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir              string `help:"Write migration files to this directory. Overrides config migrations.dir." type:"path"`
	Runner           string `help:"Migration runner file format: goose or golang-migrate."`
	Name             string `arg:"" help:"Migration name."`
	Empty            bool   `help:"Create an empty migration for manual SQL."`
	NoDown           bool   `help:"Omit the down migration section."`
	AllowDestructive bool   `help:"Allow the migration to contain destructive changes (drops, disables, comment removals)."`
	Interactive      bool   `help:"Prompt for ambiguous migration choices when stdin/stdout are TTYs."`
	NoInteractive    bool   `name:"no-interactive" help:"Disable prompts and fail closed on ambiguous migrations."`
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
	if c.Interactive && c.NoInteractive {
		return Exit(1, errors.New("--interactive and --no-interactive cannot be used together"))
	}

	interaction := app.InteractionAuto
	if c.Interactive {
		interaction = app.InteractionAlways
	}
	if c.NoInteractive {
		interaction = app.InteractionNever
	}
	var renameDecider app.RenameDecisionFunc
	if shouldPromptForRenames(interaction, g.stdin(), g.stdout()) {
		renameDecider = promptRenameCandidates(g.stdin(), g.stdout())
	}
	if _, err := app.MigrateCreateWithConfig(config, app.MigrateCreateOptions{
		Name:             c.Name,
		Dir:              c.Dir,
		Runner:           c.Runner,
		Empty:            c.Empty,
		NoDown:           c.NoDown,
		AllowDestructive: c.AllowDestructive,
		RenameDecider:    renameDecider,
		Interaction:      interaction,
	}); err != nil {
		return Exit(1, err)
	}
	return nil
}

type MigrateCheckCmd struct {
	Config                        string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir                           string `help:"Migration directory to validate. Overrides config migrations.dir." type:"path"`
	SandboxURL                    string `help:"PostgreSQL URL for replaying migrations into a disposable sandbox database."`
	SandboxURLEnv                 string `name:"sandbox-url-env" help:"Environment variable containing the sandbox PostgreSQL URL."`
	SandboxTokenCommand           string `name:"sandbox-token-command" help:"Command that prints a sandbox database auth token to stdout. The token is used as the PostgreSQL password."`
	SandboxAWSProfile             string `name:"sandbox-aws-profile" help:"AWS profile for sandbox RDS/Aurora IAM database authentication."`
	SandboxAWSRegion              string `name:"sandbox-aws-region" help:"AWS region for sandbox RDS/Aurora IAM database authentication. Defaults to the AWS config chain when omitted."`
	SandboxGCloudInstance         string `name:"sandbox-gcloud-instance" help:"Cloud SQL instance ID passed to gcloud sql generate-login-token for sandbox auth."`
	SandboxAzureCLIToken          bool   `name:"sandbox-azure-cli-token" help:"Use Azure CLI to acquire an Azure Database for PostgreSQL access token as the sandbox database password."`
	SandboxAzureDefaultCredential bool   `name:"sandbox-azure-default-credential" help:"Use Azure SDK DefaultAzureCredential to acquire an Azure Database for PostgreSQL access token as the sandbox database password."`
	SandboxAWSCLIToken            bool   `name:"sandbox-aws-cli-token" help:"Use AWS CLI to generate an RDS/Aurora PostgreSQL IAM auth token as the sandbox database password."`
	SandboxAWSIAMToken            bool   `name:"sandbox-aws-iam-token" help:"Use AWS SDK credentials to generate an RDS/Aurora PostgreSQL IAM auth token as the sandbox database password."`
	SandboxGCloudADCToken         bool   `name:"sandbox-gcloud-adc-token" help:"Use gcloud with Application Default Credentials to generate a Cloud SQL PostgreSQL IAM login token as the sandbox database password."`
	SandboxGCloudToken            bool   `name:"sandbox-gcloud-token" help:"Use gcloud to generate a Cloud SQL PostgreSQL IAM login token as the sandbox database password."`
	JSON                          bool   `help:"Emit machine-readable JSON instead of human-readable text."`
	Quiet                         bool   `help:"Suppress human-readable success output."`
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

	result, err := app.MigrateCheckWithConfig(config, app.MigrateCheckOptions{
		Dir:                           c.Dir,
		SandboxURL:                    c.SandboxURL,
		SandboxURLEnv:                 c.SandboxURLEnv,
		SandboxTokenCommand:           c.SandboxTokenCommand,
		SandboxAWSProfile:             c.SandboxAWSProfile,
		SandboxAWSRegion:              c.SandboxAWSRegion,
		SandboxGCloudInstance:         c.SandboxGCloudInstance,
		SandboxAzureCLIToken:          c.SandboxAzureCLIToken,
		SandboxAzureDefaultCredential: c.SandboxAzureDefaultCredential,
		SandboxAWSCLIToken:            c.SandboxAWSCLIToken,
		SandboxAWSIAMToken:            c.SandboxAWSIAMToken,
		SandboxGCloudADCToken:         c.SandboxGCloudADCToken,
		SandboxGCloudToken:            c.SandboxGCloudToken,
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

type MigrateApplyCmd struct {
	Config                 string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir                    string `help:"Migration directory to apply. Overrides config migrations.dir." type:"path"`
	Runner                 string `help:"Migration runner file format: goose or golang-migrate."`
	URL                    string `help:"PostgreSQL URL for the target database. Defaults to DATABASE_URL when omitted."`
	URLEnv                 string `name:"url-env" help:"Environment variable containing the PostgreSQL URL."`
	TokenCommand           string `name:"token-command" help:"Command that prints a database auth token to stdout. The token is used as the PostgreSQL password."`
	AWSProfile             string `name:"aws-profile" help:"AWS profile for RDS/Aurora IAM database authentication."`
	AWSRegion              string `name:"aws-region" help:"AWS region for RDS/Aurora IAM database authentication. Defaults to the AWS config chain when omitted."`
	GCloudInstance         string `name:"gcloud-instance" help:"Cloud SQL instance ID passed to gcloud sql generate-login-token."`
	AzureCLIToken          bool   `name:"azure-cli-token" help:"Use Azure CLI to acquire an Azure Database for PostgreSQL access token as the database password."`
	AzureDefaultCredential bool   `name:"azure-default-credential" help:"Use Azure SDK DefaultAzureCredential to acquire an Azure Database for PostgreSQL access token as the database password."`
	AWSCLIToken            bool   `name:"aws-cli-token" help:"Use AWS CLI to generate an RDS/Aurora PostgreSQL IAM auth token as the database password."`
	AWSIAMToken            bool   `name:"aws-iam-token" help:"Use AWS SDK credentials to generate an RDS/Aurora PostgreSQL IAM auth token as the database password."`
	GCloudADCToken         bool   `name:"gcloud-adc-token" help:"Use gcloud with Application Default Credentials to generate a Cloud SQL PostgreSQL IAM login token as the database password."`
	GCloudToken            bool   `name:"gcloud-token" help:"Use gcloud to generate a Cloud SQL PostgreSQL IAM login token as the database password."`
	JSON                   bool   `help:"Emit machine-readable JSON instead of human-readable text."`
	Quiet                  bool   `help:"Suppress human-readable success output."`
}

func (c *MigrateApplyCmd) Run(g *GlobalFlags) error {
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

	result, err := app.MigrateApplyWithConfig(config, app.MigrateApplyOptions{
		Dir:                    c.Dir,
		Runner:                 c.Runner,
		URL:                    c.URL,
		URLEnv:                 c.URLEnv,
		TokenCommand:           c.TokenCommand,
		AWSProfile:             c.AWSProfile,
		AWSRegion:              c.AWSRegion,
		GCloudInstance:         c.GCloudInstance,
		AzureCLIToken:          c.AzureCLIToken,
		AzureDefaultCredential: c.AzureDefaultCredential,
		AWSCLIToken:            c.AWSCLIToken,
		AWSIAMToken:            c.AWSIAMToken,
		GCloudADCToken:         c.GCloudADCToken,
		GCloudToken:            c.GCloudToken,
	})
	if err != nil {
		return Exit(1, err)
	}

	if c.JSON {
		return printJSON(g.stdout(), result)
	}
	if !printHuman(c.JSON, c.Quiet) {
		return nil
	}
	return printApply(g.stdout(), result)
}

type MigratePlanCmd struct {
	Config   string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	Dir      string `help:"Migration directory containing the previous migration. Overrides config migrations.dir." type:"path"`
	Runner   string `help:"Migration runner file format: goose or golang-migrate."`
	Snapshot string `help:"Snapshot file to diff against. Defaults to the config snapshot output."`
	JSON     bool   `help:"Emit machine-readable JSON instead of human-readable text."`
	Quiet    bool   `help:"Suppress human-readable success output."`
}

func (c *MigratePlanCmd) Run(g *GlobalFlags) error {
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

	result, err := app.MigratePlanWithConfig(config, app.MigratePlanOptions{
		Dir:      c.Dir,
		Runner:   c.Runner,
		Snapshot: c.Snapshot,
	})
	if err != nil {
		return Exit(1, err)
	}

	if c.JSON {
		return printJSON(g.stdout(), result)
	}
	if !printHuman(c.JSON, c.Quiet) {
		return nil
	}
	return printPlan(g.stdout(), result)
}

func printHuman(jsonOutput, quiet bool) bool {
	return !jsonOutput && !quiet
}

func printJSON(w io.Writer, result any) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return Exit(1, fmt.Errorf("marshal JSON: %w", err))
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func printApply(w io.Writer, result *app.MigrateApplyResult) error {
	if result == nil {
		return nil
	}
	if _, err := fmt.Fprintf(w, "migrations: %d\n", result.Count); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "applied:    %d\n", result.Applied); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "skipped:    %d\n", result.Skipped); err != nil {
		return err
	}
	if result.LastFile != "" {
		if _, err := fmt.Fprintf(w, "lastFile:   %s\n", result.LastFile); err != nil {
			return err
		}
	}
	return nil
}

func printPlan(w io.Writer, result *app.MigratePlanResult) error {
	if result == nil {
		return nil
	}
	if len(result.Changes) == 0 {
		_, err := fmt.Fprintln(w, "no changes")
		return err
	}
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Render("migration plan")
	if _, err := fmt.Fprintln(w, title); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, planSummaryTable(result)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, planChangeTable(result.Changes))
	return err
}

func planSummaryTable(result *app.MigratePlanResult) string {
	t := styledTable("Field", "Value")
	t.Row("Dialect", result.Dialect)
	if result.FromSnapshotID != "" {
		t.Row("From snapshot", result.FromSnapshotID)
	}
	t.Row("To snapshot", result.ToSnapshotID)
	t.Row("Changes", strconv.Itoa(len(result.Changes)))
	t.Row("Up statements", strconv.Itoa(len(result.Statements)))
	if len(result.Destructive) > 0 {
		t.Row("Destructive", fmt.Sprintf("%d (pass --allow-destructive to write)", len(result.Destructive)))
	}
	return t.String()
}

func planChangeTable(changes []migrate.Change) string {
	t := styledTable("Op", "Kind", "Object", "Risks", "Summary")
	for _, change := range changes {
		risks := strings.Join(change.Risks, ", ")
		t.Row(change.Op, change.Object.Kind, change.Object.Key, risks, change.Summary)
	}
	return t.String()
}

func styledTable(headers ...string) *table.Table {
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("8"))).
		Headers(headers...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			style := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return style.Bold(true).Foreground(lipgloss.Color("12"))
			}
			return style
		})
}

func shouldPromptForRenames(mode app.InteractionMode, stdin io.Reader, stdout io.Writer) bool {
	switch mode {
	case app.InteractionAlways:
		return true
	case app.InteractionNever:
		return false
	default:
		return isTerminal(stdin) && isTerminal(stdout)
	}
}

func isTerminal(value any) bool {
	file, ok := value.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func promptRenameCandidates(stdin io.Reader, stdout io.Writer) app.RenameDecisionFunc {
	return func(candidates []app.RenameCandidate) ([]app.RenameDecision, error) {
		if len(candidates) == 0 {
			return nil, nil
		}
		reader := bufio.NewReader(stdin)
		decisions := make([]app.RenameDecision, 0)
		usedSources := map[string]struct{}{}
		for _, group := range renameCandidateGroups(candidates) {
			available := make([]app.RenameCandidate, 0, len(group))
			for _, candidate := range group {
				if _, used := usedSources[candidate.FromKey]; !used {
					available = append(available, candidate)
				}
			}
			if len(available) == 0 {
				continue
			}
			selected, renamed, err := promptRenameCandidateGroup(reader, stdout, available)
			if err != nil {
				return nil, err
			}
			if !renamed {
				for _, candidate := range available {
					decisions = append(decisions, app.RenameDecision{Candidate: candidate})
				}
				continue
			}
			usedSources[selected.FromKey] = struct{}{}
			decisions = append(decisions, app.RenameDecision{Candidate: selected, Accept: true})
		}
		return decisions, nil
	}
}

func renameCandidateGroups(candidates []app.RenameCandidate) [][]app.RenameCandidate {
	groups := make([][]app.RenameCandidate, 0)
	byTarget := map[string]int{}
	for _, candidate := range candidates {
		idx, ok := byTarget[candidate.ToKey]
		if !ok {
			byTarget[candidate.ToKey] = len(groups)
			groups = append(groups, nil)
			idx = len(groups) - 1
		}
		groups[idx] = append(groups[idx], candidate)
	}
	return groups
}

func promptRenameCandidateGroup(reader *bufio.Reader, stdout io.Writer, candidates []app.RenameCandidate) (app.RenameCandidate, bool, error) {
	target := candidates[0]
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Render("ambiguous rename candidate")
	if _, err := fmt.Fprintln(stdout, title); err != nil {
		return app.RenameCandidate{}, false, err
	}
	if _, err := fmt.Fprintf(stdout, "\nIs %s %s created, or renamed from an existing %s?\n\n", target.ToKey, target.Kind, target.Kind); err != nil {
		return app.RenameCandidate{}, false, err
	}
	if _, err := fmt.Fprintln(stdout, renameCandidateDecisionTable(candidates)); err != nil {
		return app.RenameCandidate{}, false, err
	}
	if _, err := fmt.Fprintf(stdout, "\nSelect action for %s (1-%d): ", target.ToKey, len(candidates)+1); err != nil {
		return app.RenameCandidate{}, false, err
	}
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return app.RenameCandidate{}, false, err
	}
	idx, err := parseRenameDecisionSelection(answer, len(candidates)+1)
	if err != nil {
		return app.RenameCandidate{}, false, err
	}
	if idx == 0 {
		return app.RenameCandidate{}, false, nil
	}
	return candidates[idx-1], true, nil
}

func renameCandidateDecisionTable(candidates []app.RenameCandidate) string {
	t := styledTable("#", "Decision", "Kind", "From", "To", "Parent")
	target := candidates[0]
	t.Row("1", "create", target.Kind, "", target.ToKey, target.ParentKey)
	for i, candidate := range candidates {
		t.Row(strconv.Itoa(i+2), "rename", candidate.Kind, candidate.FromKey, candidate.ToKey, candidate.ParentKey)
	}
	return t.String()
}

func parseRenameDecisionSelection(answer string, count int) (int, error) {
	answer = strings.TrimSpace(strings.ToLower(answer))
	switch answer {
	case "", "c", "create", "n", "no", "none":
		return 0, nil
	}
	idx, err := strconv.Atoi(answer)
	if err != nil {
		return 0, fmt.Errorf("invalid rename selection %q", answer)
	}
	if idx < 1 || idx > count {
		return 0, fmt.Errorf("rename selection %d is out of range", idx)
	}
	if idx == 1 {
		return 0, nil
	}
	return idx - 1, nil
}
