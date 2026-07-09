package app

type CISchemaOptions struct {
	Dir                           string
	SandboxURL                    string
	SandboxURLEnv                 string
	SandboxTokenCommand           string
	SandboxAWSProfile             string
	SandboxAWSRegion              string
	SandboxGCloudInstance         string
	SandboxAzureCLIToken          bool
	SandboxAzureDefaultCredential bool
	SandboxAWSCLIToken            bool
	SandboxAWSIAMToken            bool
	SandboxGCloudADCToken         bool
	SandboxGCloudToken            bool
}

type CISchemaResult struct {
	Generate  *GenerateResult     `json:"generate"`
	Snapshot  *SnapshotResult     `json:"snapshot"`
	Migration *MigrateCheckResult `json:"migration"`
}

type CIDatabaseOptions struct {
	URL                    string
	URLEnv                 string
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

type CIDatabaseResult struct {
	Drift *DriftCheckResult `json:"drift"`
}

func CISchemaWithConfig(config *Config, opts CISchemaOptions) (*CISchemaResult, error) {
	generate, err := GenerateWithConfig(config, GenerateOptions{Check: true})
	if err != nil {
		return nil, err
	}
	snapshot, err := SnapshotWithConfig(config, SnapshotOptions{Check: true})
	if err != nil {
		return nil, err
	}
	migration, err := MigrateCheckWithConfig(config, MigrateCheckOptions{
		Dir:                           opts.Dir,
		SandboxURL:                    opts.SandboxURL,
		SandboxURLEnv:                 opts.SandboxURLEnv,
		SandboxTokenCommand:           opts.SandboxTokenCommand,
		SandboxAWSProfile:             opts.SandboxAWSProfile,
		SandboxAWSRegion:              opts.SandboxAWSRegion,
		SandboxGCloudInstance:         opts.SandboxGCloudInstance,
		SandboxAzureCLIToken:          opts.SandboxAzureCLIToken,
		SandboxAzureDefaultCredential: opts.SandboxAzureDefaultCredential,
		SandboxAWSCLIToken:            opts.SandboxAWSCLIToken,
		SandboxAWSIAMToken:            opts.SandboxAWSIAMToken,
		SandboxGCloudADCToken:         opts.SandboxGCloudADCToken,
		SandboxGCloudToken:            opts.SandboxGCloudToken,
	})
	if err != nil {
		return nil, err
	}
	return &CISchemaResult{
		Generate:  generate,
		Snapshot:  snapshot,
		Migration: migration,
	}, nil
}

func CIDatabaseWithConfig(config *Config, opts CIDatabaseOptions) (*CIDatabaseResult, error) {
	drift, err := DriftCheckWithConfig(config, DriftCheckOptions{
		URL:                    opts.URL,
		URLEnv:                 opts.URLEnv,
		TokenCommand:           opts.TokenCommand,
		AWSProfile:             opts.AWSProfile,
		AWSRegion:              opts.AWSRegion,
		GCloudInstance:         opts.GCloudInstance,
		AzureCLIToken:          opts.AzureCLIToken,
		AzureDefaultCredential: opts.AzureDefaultCredential,
		AWSCLIToken:            opts.AWSCLIToken,
		AWSIAMToken:            opts.AWSIAMToken,
		GCloudADCToken:         opts.GCloudADCToken,
		GCloudToken:            opts.GCloudToken,
	})
	if err != nil {
		return &CIDatabaseResult{Drift: drift}, err
	}
	return &CIDatabaseResult{Drift: drift}, nil
}
