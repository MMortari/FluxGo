package fluxgo

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/fx"
)

type Redis struct {
	client *redis.Client
	apm    *Apm

	subscribeBufferSize int

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}
type RedisOptions struct {
	redis.Options

	SubscribeBufferSize int
}

const defaultSubscribeBufferSize = 100

func (f *FluxGo) AddRedis(opt RedisOptions) *FluxGo {
	f.AddDependency(func(apm *Apm) *Redis {
		bufferSize := opt.SubscribeBufferSize
		if bufferSize <= 0 {
			bufferSize = defaultSubscribeBufferSize
		}

		ctx, cancel := context.WithCancel(context.Background())

		return &Redis{
			client:              redis.NewClient(&opt.Options),
			apm:                 apm,
			subscribeBufferSize: bufferSize,
			ctx:                 ctx,
			cancel:              cancel,
		}
	})
	f.AddInvoke(func(lc fx.Lifecycle, redis *Redis) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				if err := redis.connect(ctx); err != nil {
					return err
				}
				f.Log("REDIS", "Connected")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := redis.stopSubscriptions(ctx); err != nil {
					f.Log("REDIS", "Timed out waiting for subscriptions: "+err.Error())
				}

				if err := redis.disconnect(); err != nil {
					return err
				}

				f.Log("REDIS", "Disconnected")

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

func (r *Redis) Publish(ctx context.Context, topic string, payload []byte) error {
	ctx, span := r.apm.StartSpan(ctx, "redis/publish", SetAttributes(attribute.String("topic", topic)))
	defer span.End()

	if err := r.client.Publish(ctx, topic, payload).Err(); err != nil {
		span.SetError(err)
		return err
	}

	return nil
}

func (r *Redis) Subscribe(ctx context.Context, topic string) (<-chan []byte, func(), error) {
	ctx, span := r.apm.StartSpan(ctx, "redis/subscribe", SetAttributes(attribute.String("topic", topic)))
	defer span.End()

	subCtx, cancel, ok := r.trackSubscription()
	if !ok {
		err := errors.New("redis: client is shutting down")
		span.SetError(err)

		return nil, nil, err
	}

	handshakeCtx, stopHandshake := context.WithCancel(ctx)
	defer stopHandshake()
	defer context.AfterFunc(subCtx, stopHandshake)()

	pubsub := r.client.Subscribe(handshakeCtx, topic)

	if _, err := pubsub.Receive(handshakeCtx); err != nil {
		span.SetError(err)
		pubsub.Close()
		cancel()
		r.wg.Done()

		return nil, nil, err
	}

	messages := make(chan []byte, r.subscribeBufferSize)
	done := make(chan struct{})

	go func() {
		defer r.wg.Done()
		defer cancel()
		defer close(done)
		defer close(messages)
		defer pubsub.Close()

		ch := pubsub.Channel()

		for {
			select {
			case <-subCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}

				select {
				case messages <- []byte(msg.Payload):
				case <-subCtx.Done():
					return
				}
			}
		}
	}()

	return messages, func() {
		cancel()
		<-done
	}, nil
}

func (r *Redis) trackSubscription() (context.Context, context.CancelFunc, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ctx.Err() != nil {
		return nil, nil, false
	}

	subCtx, cancel := context.WithCancel(r.ctx)
	r.wg.Add(1)

	return subCtx, cancel, true
}

func (r *Redis) stopSubscriptions(ctx context.Context) error {
	r.mu.Lock()
	r.cancel()
	r.mu.Unlock()

	stopped := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(stopped)
	}()

	select {
	case <-stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
