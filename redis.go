package fluxgo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/fx"
)

type Redis struct {
	client *redis.Client
	apm    *Apm
}
type RedisOptions struct {
	redis.Options
}

func (f *FluxGo) AddRedis(opt RedisOptions) *FluxGo {
	f.AddDependency(func(apm *Apm) *Redis {
		return &Redis{client: redis.NewClient(&opt.Options), apm: apm}
	})
	f.AddInvoke(func(lc fx.Lifecycle, redis *Redis) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				if err := redis.connect(ctx); err != nil {
					return err
				}
				f.log("REDIS", "Connected")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := redis.disconnect(); err != nil {
					return err
				}
				f.log("REDIS", "Disconnected")
				return nil
			},
		})

		return nil
	})

	return f
}
func (r *Redis) connect(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
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

func (r *Redis) Get(ctx context.Context, key string) *string {
	ctx, span := r.apm.StartSpan(ctx, "redis/get", SetAttributes(attribute.String("key", key)))
	defer span.End()

	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		span.SetError(err)
		return nil
	}
	if val == "" {
		return nil
	}

	return &val
}
func (r *Redis) Store(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	ctx, span := r.apm.StartSpan(ctx, "redis/store", SetAttributes(attribute.String("key", key)))
	defer span.End()

	contentString, err := json.Marshal(value)
	if err != nil {
		span.SetError(err)
		return err
	}

	if err := r.client.Set(ctx, key, contentString, ttl).Err(); err != nil {
		span.SetError(err)
		return err
	}

	return nil
}
func (r *Redis) StoreString(ctx context.Context, key string, value string, ttl time.Duration) error {
	ctx, span := r.apm.StartSpan(ctx, "redis/storeString", SetAttributes(attribute.String("key", key)))
	defer span.End()

	if err := r.client.Set(ctx, key, value, ttl).Err(); err != nil {
		span.SetError(err)
		return err
	}

	return nil
}
func (r *Redis) Invalidate(ctx context.Context, keys []string) error {
	ctx, span := r.apm.StartSpan(ctx, "redis/invalidate", SetAttributes(attribute.StringSlice("key", keys)))
	defer span.End()

	delKeys := make([]string, 0, len(keys))

	for _, key := range keys {
		for iter := r.client.Scan(ctx, 0, key, 0).Iterator(); iter.Next(ctx); {
			delKeys = append(delKeys, iter.Val())
		}
	}

	if len(delKeys) == 0 {
		return nil
	}

	if err := r.client.Del(ctx, delKeys...).Err(); err != nil {
		span.SetError(err)
		return err
	}

	return nil
}
