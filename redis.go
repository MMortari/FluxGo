package fluxgo

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

type Redis struct {
	client *redis.Client
}
type RedisOptions struct {
	redis.Options
}

func (f *FluxGo) AddRedis(opt RedisOptions) *FluxGo {
	f.AddDependency(func() *Redis {
		return &Redis{client: redis.NewClient(&opt.Options)}
	})
	f.AddInvoke(func(lc fx.Lifecycle, redis *Redis) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return redis.connect(ctx)
			},
			OnStop: func(ctx context.Context) error {
				return redis.disconnect()
			},
		})

		return nil
	})

	return f
}
func (r *Redis) connect(ctx context.Context) error {
	if err := r.client.Ping(context.Background()).Err(); err != nil {
		return err
	}

	return nil
}
func (r *Redis) disconnect() error {
	if err := r.client.Close(); err != nil {
		return err
	}

	return nil
}
