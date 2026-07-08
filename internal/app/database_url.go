package app

import (
	"fmt"
	"os"
	"strings"
)

const defaultDatabaseURLEnv = "DATABASE_URL"

func resolveRequiredDatabaseURL(rawURL, envName, purpose string) (string, error) {
	if strings.TrimSpace(rawURL) != "" {
		return strings.TrimSpace(rawURL), nil
	}
	if envName == "" {
		envName = defaultDatabaseURLEnv
	}
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("%s URL is required; pass --url or set %s", purpose, envName)
}

func resolveOptionalDatabaseURL(rawURL, envName, purpose string) (string, error) {
	if strings.TrimSpace(rawURL) != "" {
		return strings.TrimSpace(rawURL), nil
	}
	if envName == "" {
		return "", nil
	}
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("%s URL environment variable %s is not set", purpose, envName)
}
