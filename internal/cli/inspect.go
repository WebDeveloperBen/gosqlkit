package cli

import "github.com/webdeveloperben/gosqlkit/internal/app"

type InspectCmd struct {
	Config                 string `help:"Path to gosqlkit.yaml config file. If omitted, searches current dir and parents." type:"path"`
	URL                    string `help:"Database URL to inspect. Defaults to DATABASE_URL when omitted."`
	URLEnv                 string `name:"url-env" help:"Environment variable containing the database URL."`
	Auth                   string `help:"Database authentication mode: password, token-command, azure-entra, aws-iam, or gcp-iam."`
	TokenCommand           string `name:"token-command" help:"Command that prints a database auth token to stdout. The token is used as the database password."`
	AWSProfile             string `name:"aws-profile" help:"AWS profile for RDS/Aurora IAM database authentication."`
	AWSRegion              string `name:"aws-region" help:"AWS region for RDS/Aurora IAM database authentication. Defaults to the AWS config chain when omitted."`
	GCloudInstance         string `name:"gcloud-instance" help:"Cloud SQL instance ID passed to gcloud sql generate-login-token."`
	Out                    string `help:"Write inspected snapshot JSON to path instead of stdout." type:"path"`
	AzureCLIToken          bool   `name:"azure-cli-token" help:"Use Azure CLI to acquire an Azure Database for PostgreSQL access token as the database password."`
	AzureDefaultCredential bool   `name:"azure-default-credential" help:"Use Azure SDK DefaultAzureCredential to acquire an Azure Database for PostgreSQL access token as the database password."`
	AWSCLIToken            bool   `name:"aws-cli-token" help:"Use AWS CLI to generate an RDS/Aurora PostgreSQL IAM auth token as the database password."`
	AWSIAMToken            bool   `name:"aws-iam-token" help:"Use AWS SDK credentials to generate an RDS/Aurora PostgreSQL IAM auth token as the database password."`
	GCloudADCToken         bool   `name:"gcloud-adc-token" help:"Use gcloud with Application Default Credentials to generate a Cloud SQL PostgreSQL IAM login token as the database password."`
	GCloudToken            bool   `name:"gcloud-token" help:"Use gcloud to generate a Cloud SQL PostgreSQL IAM login token as the database password."`
	JSON                   bool   `help:"Print inspected snapshot JSON to stdout even when --out is set. Snapshot JSON is printed by default when --out is omitted."`
	DriftProjection        bool   `name:"drift-projection" help:"Emit the normalised database shape used by drift checks."`
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
		URL:                    c.URL,
		URLEnv:                 c.URLEnv,
		Auth:                   c.Auth,
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
		Out:                    c.Out,
		Stdout:                 g.stdout(),
		JSON:                   c.JSON,
		DriftProjection:        c.DriftProjection,
	}); err != nil {
		return Exit(1, err)
	}
	return nil
}
