package kit

import (
	"context"
	"time"
)

type AccessToken struct {
	ExpiresAt time.Time
	Value     string
}

type TokenProvider interface {
	Token(context.Context) (AccessToken, error)
}
