package fluxgo

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

type RedisOptions struct {
	redis.Options
}

func (f *FluxGo) AddRedis(opt RedisOptions) *FluxGo {
	client := redis.NewClient(&opt.Options)

	f.AddInvoke(fx.Invoke(func(lc fx.Lifecycle) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return redisConnect(client)
			},
			OnStop: func(ctx context.Context) error {
				return redisDisconnect(client)
			},
		})

		return nil
	}))

	return f
}
func redisConnect(client *redis.Client) error {
	if err := client.Ping(context.Background()).Err(); err != nil {
		return err
	}

	return nil
}
func redisDisconnect(client *redis.Client) error {
	err := client.Close()

	if err != nil {
		return err
	}

	return nil
}
