package tooling

import (
	"context"
	"strings"
	"testing"
)

type retryPasswordProvider struct {
	passwords int
	refreshes int
}

func (p *retryPasswordProvider) Password(context.Context) (string, error) {
	p.passwords++
	return "token", nil
}

func (p *retryPasswordProvider) Refresh(context.Context) (string, error) {
	p.refreshes++
	return "refreshed-token", nil
}

func TestOpenWithOptionsRefreshesProviderBeforeRetry(t *testing.T) {
	provider := &retryPasswordProvider{}
	_, err := OpenWithOptions(context.Background(), ConnectionOptions{
		URL:              "postgres://user@127.0.0.1:1/app",
		PasswordProvider: provider,
		ConnectAttempts:  2,
	})
	if err == nil || !strings.Contains(err.Error(), "connect to PostgreSQL") {
		t.Fatalf("OpenWithOptions error = %v", err)
	}
	if provider.passwords != 1 || provider.refreshes != 1 {
		t.Fatalf("provider calls = password %d, refresh %d; want 1, 1", provider.passwords, provider.refreshes)
	}
}

func TestOpenWithOptionsDefaultsToOneAttempt(t *testing.T) {
	provider := &retryPasswordProvider{}
	_, err := OpenWithOptions(context.Background(), ConnectionOptions{
		URL:              "postgres://user@127.0.0.1:1/app",
		PasswordProvider: provider,
	})
	if err == nil {
		t.Fatal("OpenWithOptions error = nil")
	}
	if provider.passwords != 1 || provider.refreshes != 0 {
		t.Fatalf("provider calls = password %d, refresh %d; want 1, 0", provider.passwords, provider.refreshes)
	}
}
