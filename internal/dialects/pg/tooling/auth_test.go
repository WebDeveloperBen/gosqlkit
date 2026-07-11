package tooling

import (
	"context"
	"testing"
	"time"
)

type testTokenProvider struct {
	tokens []Token
	calls  int
}

func (p *testTokenProvider) Token(context.Context) (Token, error) {
	token := p.tokens[p.calls]
	p.calls++
	return token, nil
}

func TestRefreshingPasswordProviderCachesUsableToken(t *testing.T) {
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	provider := &testTokenProvider{tokens: []Token{{Value: "first", ExpiresAt: now.Add(time.Hour)}}}
	passwords := NewRefreshingPasswordProvider(provider, time.Minute)
	passwords.now = func() time.Time { return now }

	for range 2 {
		password, err := passwords.Password(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if password != "first" {
			t.Fatalf("password = %q, want first", password)
		}
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
}

func TestRefreshingPasswordProviderRefreshesExpiringToken(t *testing.T) {
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	provider := &testTokenProvider{tokens: []Token{
		{Value: "first", ExpiresAt: now.Add(30 * time.Second)},
		{Value: "second", ExpiresAt: now.Add(time.Hour)},
	}}
	passwords := NewRefreshingPasswordProvider(provider, time.Minute)
	passwords.now = func() time.Time { return now }

	password, err := passwords.Password(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if password != "first" {
		t.Fatalf("password = %q, want first", password)
	}
	password, err = passwords.Password(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if password != "second" {
		t.Fatalf("password = %q, want second", password)
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.calls)
	}
}

func TestRefreshingPasswordProviderDoesNotCacheTokenWithoutExpiry(t *testing.T) {
	provider := &testTokenProvider{tokens: []Token{{Value: "first"}, {Value: "second"}}}
	passwords := NewRefreshingPasswordProvider(provider, time.Minute)

	if _, err := passwords.Password(context.Background()); err != nil {
		t.Fatal(err)
	}
	password, err := passwords.Password(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if password != "second" || provider.calls != 2 {
		t.Fatalf("password = %q, calls = %d", password, provider.calls)
	}
}
