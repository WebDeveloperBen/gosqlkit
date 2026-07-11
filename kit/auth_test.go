package kit_test

import (
	"context"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/kit"
)

type testTokenProvider struct{}

func (testTokenProvider) Token(context.Context) (kit.AccessToken, error) {
	return kit.AccessToken{Value: "custom-token", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func TestTokenProviderIsPublicExtensionBoundary(t *testing.T) {
	var _ kit.TokenProvider = testTokenProvider{}
	if token, err := (testTokenProvider{}).Token(context.Background()); err != nil || token.Value != "custom-token" {
		t.Fatalf("token = %#v, error = %v", token, err)
	}
}
