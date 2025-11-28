package fluxgo

import (
	"context"
	"time"
)

type ICache interface {
	Get(ctx context.Context, key string) *string
	Store(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Invalidate(ctx context.Context, keys []string) error
}
