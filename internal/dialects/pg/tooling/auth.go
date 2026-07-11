package tooling

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

type Token struct {
	ExpiresAt time.Time
	Value     string
}

type TokenProvider interface {
	Token(context.Context) (Token, error)
}

type RefreshingPasswordProvider struct {
	provider      TokenProvider
	now           func() time.Time
	token         Token
	refreshBefore time.Duration
	mu            sync.Mutex
}

func NewRefreshingPasswordProvider(provider TokenProvider, refreshBefore time.Duration) *RefreshingPasswordProvider {
	return &RefreshingPasswordProvider{
		provider:      provider,
		refreshBefore: refreshBefore,
		now:           time.Now,
	}
}

func (p *RefreshingPasswordProvider) Password(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.valid() {
		return p.token.Value, nil
	}
	return p.refresh(ctx)
}

func (p *RefreshingPasswordProvider) Refresh(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.refresh(ctx)
}

func (p *RefreshingPasswordProvider) valid() bool {
	return strings.TrimSpace(p.token.Value) != "" && !p.token.ExpiresAt.IsZero() && p.token.ExpiresAt.After(p.now().Add(p.refreshBefore))
}

func (p *RefreshingPasswordProvider) refresh(ctx context.Context) (string, error) {
	if p.provider == nil {
		return "", errors.New("database token provider is required")
	}
	token, err := p.provider.Token(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token.Value) == "" {
		return "", errors.New("database token provider produced an empty token")
	}
	p.token = token
	return token.Value, nil
}
