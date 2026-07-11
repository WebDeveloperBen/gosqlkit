package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestSplitCommandLine(t *testing.T) {
	got, err := splitCommandLine(`az account get-access-token --query "accessToken" --resource-type 'oss-rdbms' escaped\ value`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"az", "account", "get-access-token", "--query", "accessToken", "--resource-type", "oss-rdbms", "escaped value"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split command line = %#v, want %#v", got, want)
	}
}

func TestSplitCommandLineRejectsUnterminatedQuote(t *testing.T) {
	_, err := splitCommandLine(`token-command "unterminated`)
	if err == nil || !strings.Contains(err.Error(), "unterminated quoted string") {
		t.Fatalf("expected unterminated quote error, got %v", err)
	}
}

func TestCommandPasswordProviderRunsCommand(t *testing.T) {
	command := writeTokenCommand(t, "secret-token", 0)

	password, err := (commandPasswordProvider{command: command}).Password(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if password != "secret-token" {
		t.Fatalf("password = %q", password)
	}
}

func TestCommandPasswordProviderRedactsTokenOutputOnFailure(t *testing.T) {
	command := writeTokenCommand(t, "secret-token", 1)

	_, err := (commandPasswordProvider{command: command}).Password(context.Background())
	if err == nil {
		t.Fatal("expected command error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("token leaked in error: %v", err)
	}
}

func TestAzureCLIPasswordProviderUsesAzurePostgresTokenCommand(t *testing.T) {
	provider := databasePasswordProvider("", databaseAuthOptions{AzureCLIToken: true})
	if _, ok := provider.(azureCLIPasswordProvider); !ok {
		t.Fatalf("provider = %T, want azureCLIPasswordProvider", provider)
	}

	parts, err := splitCommandLine(azureCLITokenCommand)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"az", "account", "get-access-token", "--resource-type", "oss-rdbms", "--query", "accessToken", "-o", "tsv"}
	if !reflect.DeepEqual(parts, want) {
		t.Fatalf("azure command = %#v, want %#v", parts, want)
	}
}

func TestValidateDatabaseAuthOptionsRejectsMultipleTokenProviders(t *testing.T) {
	err := validateDatabaseAuthOptions(databaseAuthOptions{
		TokenCommand:  "print-token",
		AzureCLIToken: true,
	})
	if err == nil || !strings.Contains(err.Error(), "only one database token provider") {
		t.Fatalf("expected token provider conflict, got %v", err)
	}
}

func TestNormaliseDatabaseAuthOptions(t *testing.T) {
	tests := []struct {
		check func(*testing.T, databaseAuthOptions)
		name  string
		auth  databaseAuthOptions
	}{
		{
			name: "password",
			auth: databaseAuthOptions{Auth: databaseAuthPassword},
		},
		{
			name: "token command",
			auth: databaseAuthOptions{Auth: databaseAuthTokenCommand, TokenCommand: "print-token"},
		},
		{
			name: "azure entra",
			auth: databaseAuthOptions{Auth: databaseAuthAzureEntra},
			check: func(t *testing.T, got databaseAuthOptions) {
				t.Helper()
				if !got.AzureDefaultCredential {
					t.Fatal("AzureDefaultCredential = false, want true")
				}
			},
		},
		{
			name: "aws iam",
			auth: databaseAuthOptions{Auth: databaseAuthAWSIAM, AWSRegion: "ap-southeast-2"},
			check: func(t *testing.T, got databaseAuthOptions) {
				t.Helper()
				if !got.AWSIAMToken {
					t.Fatal("AWSIAMToken = false, want true")
				}
			},
		},
		{
			name: "gcp iam",
			auth: databaseAuthOptions{Auth: databaseAuthGCPIAM, GCloudInstance: "project:region:instance"},
			check: func(t *testing.T, got databaseAuthOptions) {
				t.Helper()
				if !got.GCloudADCToken {
					t.Fatal("GCloudADCToken = false, want true")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normaliseDatabaseAuthOptions(tt.auth)
			if err != nil {
				t.Fatal(err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestNormaliseDatabaseAuthOptionsRejectsInvalidModes(t *testing.T) {
	for _, auth := range []databaseAuthOptions{
		{Auth: "unknown"},
		{Auth: databaseAuthTokenCommand},
		{Auth: databaseAuthAWSIAM, AzureCLIToken: true},
		{Auth: databaseAuthPassword, AWSRegion: "ap-southeast-2"},
	} {
		if _, err := normaliseDatabaseAuthOptions(auth); err == nil {
			t.Fatalf("normaliseDatabaseAuthOptions(%+v) error = nil", auth)
		}
	}
}

func TestAzureDefaultCredentialPasswordProviderRequestsPostgresScope(t *testing.T) {
	credential := &fakeAzureCredential{accessValue: "azure-sdk-access"}
	password, err := (azureDefaultCredentialPasswordProvider{credential: credential}).Password(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if password != "azure-sdk-access" {
		t.Fatalf("password = %q", password)
	}
	want := []string{azurePostgresScope}
	if !reflect.DeepEqual(credential.scopes, want) {
		t.Fatalf("scopes = %#v, want %#v", credential.scopes, want)
	}
}

func TestDatabasePasswordProviderUsesAzureDefaultCredential(t *testing.T) {
	provider := databasePasswordProvider("", databaseAuthOptions{AzureDefaultCredential: true})
	if _, ok := provider.(azureDefaultCredentialPasswordProvider); !ok {
		t.Fatalf("provider = %T, want azureDefaultCredentialPasswordProvider", provider)
	}
}

func TestDatabasePasswordProviderUsesAWSIAMToken(t *testing.T) {
	rawURL := "postgres://db_user@postgresmydb.123456789012.us-east-1.rds.amazonaws.com:5432/app"
	provider := databasePasswordProvider(rawURL, databaseAuthOptions{
		AWSIAMToken: true,
		AWSProfile:  "dev",
		AWSRegion:   "us-east-1",
	})
	got, ok := provider.(awsIAMPasswordProvider)
	if !ok {
		t.Fatalf("provider = %T, want awsIAMPasswordProvider", provider)
	}
	if got.rawURL != rawURL || got.region != "us-east-1" || got.profile != "dev" {
		t.Fatalf("provider = %#v", got)
	}
}

func TestDatabasePasswordProviderUsesAWSCLIToken(t *testing.T) {
	rawURL := "postgres://db_user@postgresmydb.123456789012.us-east-1.rds.amazonaws.com:5432/app"
	provider := databasePasswordProvider(rawURL, databaseAuthOptions{
		AWSCLIToken: true,
		AWSProfile:  "dev",
		AWSRegion:   "us-east-1",
	})
	got, ok := provider.(awsCLIPasswordProvider)
	if !ok {
		t.Fatalf("provider = %T, want awsCLIPasswordProvider", provider)
	}
	if got.rawURL != rawURL || got.region != "us-east-1" || got.profile != "dev" {
		t.Fatalf("provider = %#v", got)
	}
}

func TestDatabasePasswordProviderUsesGCloudToken(t *testing.T) {
	provider := databasePasswordProvider("", databaseAuthOptions{
		GCloudToken:    true,
		GCloudInstance: "app-prod",
	})
	got, ok := provider.(gcloudPasswordProvider)
	if !ok {
		t.Fatalf("provider = %T, want gcloudPasswordProvider", provider)
	}
	if got.instance != "app-prod" || got.applicationDefault {
		t.Fatalf("provider = %#v", got)
	}
}

func TestDatabasePasswordProviderUsesGCloudADCToken(t *testing.T) {
	provider := databasePasswordProvider("", databaseAuthOptions{
		GCloudADCToken: true,
		GCloudInstance: "app-prod",
	})
	got, ok := provider.(gcloudPasswordProvider)
	if !ok {
		t.Fatalf("provider = %T, want gcloudPasswordProvider", provider)
	}
	if got.instance != "app-prod" || !got.applicationDefault {
		t.Fatalf("provider = %#v", got)
	}
}

func TestResolvePostgresAuthTarget(t *testing.T) {
	target, err := resolvePostgresAuthTarget("postgres://db_user@postgresmydb.123456789012.us-east-1.rds.amazonaws.com:5432/app")
	if err != nil {
		t.Fatal(err)
	}
	if target.endpoint != "postgresmydb.123456789012.us-east-1.rds.amazonaws.com:5432" {
		t.Fatalf("endpoint = %q", target.endpoint)
	}
	if target.user != "db_user" {
		t.Fatalf("user = %q", target.user)
	}
	if target.host != "postgresmydb.123456789012.us-east-1.rds.amazonaws.com" {
		t.Fatalf("host = %q", target.host)
	}
	if target.port != 5432 {
		t.Fatalf("port = %d", target.port)
	}

	target, err = resolvePostgresAuthTarget("host=postgresmydb.123456789012.us-east-1.rds.amazonaws.com user=db_user dbname=app")
	if err != nil {
		t.Fatal(err)
	}
	if target.endpoint != "postgresmydb.123456789012.us-east-1.rds.amazonaws.com:5432" {
		t.Fatalf("keyword endpoint = %q", target.endpoint)
	}
}

func TestAWSCLITokenArgs(t *testing.T) {
	target := postgresAuthTarget{
		host: "postgresmydb.123456789012.us-east-1.rds.amazonaws.com",
		port: 5432,
		user: "db_user",
	}
	got := awsCLITokenArgs(target, "us-east-1", "dev")
	want := []string{
		"aws",
		"rds",
		"generate-db-auth-token",
		"--hostname",
		"postgresmydb.123456789012.us-east-1.rds.amazonaws.com",
		"--port",
		"5432",
		"--username",
		"db_user",
		"--no-cli-pager",
		"--region",
		"us-east-1",
		"--profile",
		"dev",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestGCloudLoginTokenArgs(t *testing.T) {
	got := gcloudLoginTokenArgs("app-prod", true)
	want := []string{
		"gcloud",
		"sql",
		"generate-login-token",
		"--quiet",
		"--instance",
		"app-prod",
		"--application-default-credential",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestAWSIAMPasswordProviderBuildsTokenForConnectionTarget(t *testing.T) {
	credentials := fakeAWSCredentialsProvider{}
	var gotEndpoint string
	var gotRegion string
	var gotUser string
	var gotCredentials aws.CredentialsProvider
	var gotProfile string
	password, err := (awsIAMPasswordProvider{
		credentials: credentials,
		loadConfig: func(_ context.Context, _ string, profile string) (aws.Config, error) {
			gotProfile = profile
			return aws.Config{Region: "ignored"}, nil
		},
		buildToken: func(_ context.Context, endpoint, region, dbUser string, creds aws.CredentialsProvider) (string, error) {
			gotEndpoint = endpoint
			gotRegion = region
			gotUser = dbUser
			gotCredentials = creds
			return "aws-iam-access", nil
		},
		rawURL:  "postgres://db_user@postgresmydb.123456789012.us-east-1.rds.amazonaws.com:5432/app",
		region:  "ap-southeast-2",
		profile: "dev",
	}).Password(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if password != "aws-iam-access" {
		t.Fatalf("password = %q", password)
	}
	if gotEndpoint != "postgresmydb.123456789012.us-east-1.rds.amazonaws.com:5432" ||
		gotRegion != "ap-southeast-2" ||
		gotUser != "db_user" ||
		gotCredentials == nil ||
		gotProfile != "dev" {
		t.Fatalf("token inputs endpoint=%q region=%q user=%q credentials=%T profile=%q", gotEndpoint, gotRegion, gotUser, gotCredentials, gotProfile)
	}
}

func TestAWSIAMPasswordProviderRequiresRegion(t *testing.T) {
	_, err := (awsIAMPasswordProvider{
		credentials: fakeAWSCredentialsProvider{},
		loadConfig: func(context.Context, string, string) (aws.Config, error) {
			return aws.Config{}, nil
		},
		buildToken: func(context.Context, string, string, string, aws.CredentialsProvider) (string, error) {
			t.Fatal("buildToken should not be called")
			return "", nil
		},
		rawURL: "postgres://db_user@postgresmydb.123456789012.us-east-1.rds.amazonaws.com:5432/app",
	}).Password(context.Background())
	if err == nil || !strings.Contains(err.Error(), "aws region is required") {
		t.Fatalf("expected region error, got %v", err)
	}
}

func TestValidateDatabaseAuthOptionsRejectsAzureCredentialConflicts(t *testing.T) {
	err := validateDatabaseAuthOptions(databaseAuthOptions{
		AzureCLIToken:          true,
		AzureDefaultCredential: true,
	})
	if err == nil || !strings.Contains(err.Error(), "only one database token provider") {
		t.Fatalf("expected token provider conflict, got %v", err)
	}
}

func TestValidateDatabaseAuthOptionsRejectsAWSIAMConflicts(t *testing.T) {
	err := validateDatabaseAuthOptions(databaseAuthOptions{
		TokenCommand: "print-token",
		AWSIAMToken:  true,
		AWSRegion:    "us-east-1",
	})
	if err == nil || !strings.Contains(err.Error(), "only one database token provider") {
		t.Fatalf("expected token provider conflict, got %v", err)
	}
}

func TestValidateDatabaseAuthOptionsRejectsAWSCLIConflicts(t *testing.T) {
	err := validateDatabaseAuthOptions(databaseAuthOptions{
		TokenCommand: "print-token",
		AWSCLIToken:  true,
		AWSRegion:    "us-east-1",
	})
	if err == nil || !strings.Contains(err.Error(), "only one database token provider") {
		t.Fatalf("expected token provider conflict, got %v", err)
	}
}

func TestValidateDatabaseAuthOptionsRejectsGCloudConflicts(t *testing.T) {
	err := validateDatabaseAuthOptions(databaseAuthOptions{
		TokenCommand: "print-token",
		GCloudToken:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "only one database token provider") {
		t.Fatalf("expected token provider conflict, got %v", err)
	}

	err = validateDatabaseAuthOptions(databaseAuthOptions{
		GCloudADCToken: true,
		GCloudToken:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "only one database token provider") {
		t.Fatalf("expected token provider conflict, got %v", err)
	}
}

type fakeAzureCredential struct {
	accessValue string
	scopes      []string
}

func (c *fakeAzureCredential) GetToken(_ context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.scopes = append([]string(nil), opts.Scopes...)
	return azcore.AccessToken{
		Token:     c.accessValue,
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

type fakeAWSCredentialsProvider struct{}

func (fakeAWSCredentialsProvider) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-access-secret",
		Source:          "test",
	}, nil
}

func writeTokenCommand(t *testing.T, token string, exitCode int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token.sh")
	content := "#!/bin/sh\nprintf '%s\\n' '" + token + "'\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// #nosec G302 -- test helper intentionally creates an executable token-command script.
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
