package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	rdsauth "github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/jackc/pgx/v5"
	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
)

type databaseAuthOptions struct {
	TokenCommand           string
	AWSProfile             string
	AWSRegion              string
	GCloudInstance         string
	AzureCLIToken          bool
	AzureDefaultCredential bool
	AWSCLIToken            bool
	AWSIAMToken            bool
	GCloudADCToken         bool
	GCloudToken            bool
}

func postgresConnectionOptions(rawURL string, auth databaseAuthOptions) pgtooling.ConnectionOptions {
	return pgtooling.ConnectionOptions{
		URL:              rawURL,
		PasswordProvider: databasePasswordProvider(rawURL, auth),
	}
}

func databasePasswordProvider(rawURL string, auth databaseAuthOptions) pgtooling.PasswordProvider {
	if auth.AzureCLIToken {
		return azureCLIPasswordProvider{}
	}
	if auth.AzureDefaultCredential {
		return azureDefaultCredentialPasswordProvider{}
	}
	if auth.AWSCLIToken {
		return awsCLIPasswordProvider{rawURL: rawURL, region: auth.AWSRegion, profile: auth.AWSProfile}
	}
	if auth.AWSIAMToken {
		return awsIAMPasswordProvider{rawURL: rawURL, region: auth.AWSRegion, profile: auth.AWSProfile}
	}
	if auth.GCloudToken {
		return gcloudPasswordProvider{instance: auth.GCloudInstance}
	}
	if auth.GCloudADCToken {
		return gcloudPasswordProvider{applicationDefault: true, instance: auth.GCloudInstance}
	}
	if strings.TrimSpace(auth.TokenCommand) == "" {
		return nil
	}
	return commandPasswordProvider{command: auth.TokenCommand}
}

func validateDatabaseAuthOptions(auth databaseAuthOptions) error {
	count := 0
	if strings.TrimSpace(auth.TokenCommand) != "" {
		count++
	}
	if auth.AzureCLIToken {
		count++
	}
	if auth.AzureDefaultCredential {
		count++
	}
	if auth.AWSCLIToken {
		count++
	}
	if auth.AWSIAMToken {
		count++
	}
	if auth.GCloudToken {
		count++
	}
	if auth.GCloudADCToken {
		count++
	}
	if count > 1 {
		return errors.New("only one database token provider can be configured")
	}
	return nil
}

const (
	azureCLITokenCommand = "az account get-access-token --resource-type oss-rdbms --query accessToken -o tsv"
	azurePostgresScope   = "https://ossrdbms-aad.database.windows.net/.default"
)

type azureCLIPasswordProvider struct{}

func (p azureCLIPasswordProvider) Password(ctx context.Context) (string, error) {
	return (commandPasswordProvider{command: azureCLITokenCommand}).Password(ctx)
}

type azureDefaultCredentialPasswordProvider struct {
	credential azcore.TokenCredential
}

func (p azureDefaultCredentialPasswordProvider) Password(ctx context.Context) (string, error) {
	credential := p.credential
	if credential == nil {
		var err error
		credential, err = azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return "", fmt.Errorf("create Azure default credential: %w", err)
		}
	}
	token, err := credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{azurePostgresScope},
	})
	if err != nil {
		return "", fmt.Errorf("get Azure PostgreSQL access token: %w", err)
	}
	if strings.TrimSpace(token.Token) == "" {
		return "", errors.New("azure default credential produced an empty access token")
	}
	return token.Token, nil
}

type awsConfigLoader func(context.Context, string, string) (aws.Config, error)

type awsAuthTokenBuilder func(context.Context, string, string, string, aws.CredentialsProvider) (string, error)

type awsIAMPasswordProvider struct {
	credentials aws.CredentialsProvider
	loadConfig  awsConfigLoader
	buildToken  awsAuthTokenBuilder
	rawURL      string
	region      string
	profile     string
}

func (p awsIAMPasswordProvider) Password(ctx context.Context) (string, error) {
	target, err := resolvePostgresAuthTarget(p.rawURL)
	if err != nil {
		return "", err
	}
	region := strings.TrimSpace(p.region)
	loadConfig := p.loadConfig
	if loadConfig == nil {
		loadConfig = loadAWSDefaultConfig
	}
	cfg, err := loadConfig(ctx, region, strings.TrimSpace(p.profile))
	if err != nil {
		return "", err
	}
	if region == "" {
		region = strings.TrimSpace(cfg.Region)
	}
	if region == "" {
		return "", errors.New("aws region is required for IAM database auth; pass --aws-region or configure AWS_REGION")
	}
	credentials := p.credentials
	if credentials == nil {
		credentials = cfg.Credentials
	}
	buildToken := p.buildToken
	if buildToken == nil {
		buildToken = buildAWSAuthToken
	}
	authToken, err := buildToken(ctx, target.endpoint, region, target.user, credentials)
	if err != nil {
		return "", fmt.Errorf("build AWS RDS IAM auth token: %w", err)
	}
	if strings.TrimSpace(authToken) == "" {
		return "", errors.New("aws rds IAM auth token provider produced an empty token")
	}
	return authToken, nil
}

func buildAWSAuthToken(ctx context.Context, endpoint, region, user string, credentials aws.CredentialsProvider) (string, error) {
	return rdsauth.BuildAuthToken(ctx, endpoint, region, user, credentials)
}

func loadAWSDefaultConfig(ctx context.Context, region, profile string) (aws.Config, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if strings.TrimSpace(region) != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	if strings.TrimSpace(profile) != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

type awsCLIPasswordProvider struct {
	rawURL  string
	region  string
	profile string
}

func (p awsCLIPasswordProvider) Password(ctx context.Context) (string, error) {
	target, err := resolvePostgresAuthTarget(p.rawURL)
	if err != nil {
		return "", err
	}
	return commandPasswordProvider{command: strings.Join(awsCLITokenArgs(target, p.region, p.profile), " ")}.Password(ctx)
}

func awsCLITokenArgs(target postgresAuthTarget, region, profile string) []string {
	args := []string{
		"aws",
		"rds",
		"generate-db-auth-token",
		"--hostname",
		target.host,
		"--port",
		strconv.Itoa(int(target.port)),
		"--username",
		target.user,
		"--no-cli-pager",
	}
	if strings.TrimSpace(region) != "" {
		args = append(args, "--region", strings.TrimSpace(region))
	}
	if strings.TrimSpace(profile) != "" {
		args = append(args, "--profile", strings.TrimSpace(profile))
	}
	return args
}

type gcloudPasswordProvider struct {
	instance           string
	applicationDefault bool
}

func (p gcloudPasswordProvider) Password(ctx context.Context) (string, error) {
	return commandPasswordProvider{command: strings.Join(gcloudLoginTokenArgs(p.instance, p.applicationDefault), " ")}.Password(ctx)
}

func gcloudLoginTokenArgs(instance string, applicationDefault bool) []string {
	args := []string{
		"gcloud",
		"sql",
		"generate-login-token",
		"--quiet",
	}
	if strings.TrimSpace(instance) != "" {
		args = append(args, "--instance", strings.TrimSpace(instance))
	}
	if applicationDefault {
		args = append(args, "--application-default-credential")
	}
	return args
}

type postgresAuthTarget struct {
	endpoint string
	host     string
	user     string
	port     uint16
}

func resolvePostgresAuthTarget(rawURL string) (postgresAuthTarget, error) {
	config, err := pgx.ParseConfig(rawURL)
	if err != nil {
		return postgresAuthTarget{}, fmt.Errorf("parse PostgreSQL URL for auth target: %w", err)
	}
	host := strings.TrimSpace(config.Host)
	if host == "" {
		return postgresAuthTarget{}, errors.New("postgres host is required for token auth")
	}
	if strings.HasPrefix(host, "/") {
		return postgresAuthTarget{}, errors.New("postgres TCP host is required for token auth")
	}
	user := strings.TrimSpace(config.User)
	if user == "" {
		return postgresAuthTarget{}, errors.New("postgres user is required for token auth")
	}
	port := config.Port
	if port == 0 {
		port = 5432
	}
	return postgresAuthTarget{
		endpoint: net.JoinHostPort(host, strconv.Itoa(int(port))),
		host:     host,
		port:     port,
		user:     user,
	}, nil
}

type commandPasswordProvider struct {
	command string
}

func (p commandPasswordProvider) Password(ctx context.Context) (string, error) {
	parts, err := splitCommandLine(p.command)
	if err != nil {
		return "", fmt.Errorf("parse token command: %w", err)
	}
	if len(parts) == 0 {
		return "", errors.New("token command is required")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...) // #nosec G204 -- command is explicit user CLI input for database token acquisition.
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("run token command: %w", err)
	}
	token := strings.TrimSpace(stdout.String())
	if token == "" {
		return "", errors.New("token command produced no stdout")
	}
	return token, nil
}

func splitCommandLine(command string) ([]string, error) {
	var out []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range command {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if current.Len() > 0 {
				out = append(out, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if escaped {
		return nil, errors.New("trailing escape in command")
	}
	if quote != 0 {
		return nil, errors.New("unterminated quoted string in command")
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out, nil
}
