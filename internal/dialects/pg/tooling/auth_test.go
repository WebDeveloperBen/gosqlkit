package tooling

import (
	"context"
	"testing"
	"time"

	"golang.org/x/oauth2"
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

type testOAuth2TokenSource struct {
	token *oauth2.Token
}

func (s testOAuth2TokenSource) Token() (*oauth2.Token, error) {
	return s.token, nil
}

func TestOAuth2TokenProviderPreservesExpiry(t *testing.T) {
	expires := time.Date(2026, 7, 11, 1, 0, 0, 0, time.UTC)
	token, err := (OAuth2TokenProvider{Source: testOAuth2TokenSource{token: &oauth2.Token{
		AccessToken: "oauth-access",
		Expiry:      expires,
	}}}).Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token.Value != "oauth-access" || !token.ExpiresAt.Equal(expires) {
		t.Fatalf("token = %#v", token)
	}
}

func TestOAuth2TokenProviderRejectsMissingSourceAndEmptyToken(t *testing.T) {
	if _, err := (OAuth2TokenProvider{}).Token(context.Background()); err == nil {
		t.Fatal("missing source error = nil")
	}
	if _, err := (OAuth2TokenProvider{Source: testOAuth2TokenSource{token: &oauth2.Token{}}}).Token(context.Background()); err == nil {
		t.Fatal("empty token error = nil")
	}
}
