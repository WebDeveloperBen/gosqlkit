package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
)

type databaseAuthOptions struct {
	TokenCommand string
}

func postgresConnectionOptions(rawURL string, auth databaseAuthOptions) pgtooling.ConnectionOptions {
	return pgtooling.ConnectionOptions{
		URL:              rawURL,
		PasswordProvider: databasePasswordProvider(auth),
	}
}

func databasePasswordProvider(auth databaseAuthOptions) pgtooling.PasswordProvider {
	if strings.TrimSpace(auth.TokenCommand) == "" {
		return nil
	}
	return commandPasswordProvider{command: auth.TokenCommand}
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
