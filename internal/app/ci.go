package app

type CISchemaOptions struct {
	Dir                 string
	SandboxURL          string
	SandboxURLEnv       string
	SandboxTokenCommand string
}

type CISchemaResult struct {
	Generate  *GenerateResult     `json:"generate"`
	Snapshot  *SnapshotResult     `json:"snapshot"`
	Migration *MigrateCheckResult `json:"migration"`
}

type CIDatabaseOptions struct {
	URL          string
	URLEnv       string
	TokenCommand string
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
		Dir:                 opts.Dir,
		SandboxURL:          opts.SandboxURL,
		SandboxURLEnv:       opts.SandboxURLEnv,
		SandboxTokenCommand: opts.SandboxTokenCommand,
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
		URL:          opts.URL,
		URLEnv:       opts.URLEnv,
		TokenCommand: opts.TokenCommand,
	})
	if err != nil {
		return &CIDatabaseResult{Drift: drift}, err
	}
	return &CIDatabaseResult{Drift: drift}, nil
}
