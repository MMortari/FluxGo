package fluxgo

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ITransport é o contrato pub/sub que o Redis precisa satisfazer. Declarado
// aqui para travar a assinatura sem expor mais uma interface na API pública.
type ITransport interface {
	Publish(ctx context.Context, topic string, payload []byte) error
	Subscribe(ctx context.Context, topic string) (<-chan []byte, func(), error)
}

var _ ITransport = (*Redis)(nil)

// newTestRedis monta um Redis apontando para um servidor em memória, com o
// contexto do provider que AddRedis normalmente criaria.
func newTestRedis(t *testing.T) (*Redis, *miniredis.Miniredis) {
	t.Helper()

	srv := miniredis.RunT(t)
	ctx, cancel := context.WithCancel(context.Background())

	r := &Redis{
		client:              redis.NewClient(&redis.Options{Addr: srv.Addr()}),
		apm:                 &Apm{},
		subscribeBufferSize: defaultSubscribeBufferSize,
		ctx:                 ctx,
		cancel:              cancel,
	}

	t.Cleanup(func() {
		cancel()
		_ = r.client.Close()
	})

	return r, srv
}

func TestRedis_AddRedis(t *testing.T) {
	t.Run("Should default the subscribe buffer when unset", func(t *testing.T) {
		flux := New(FluxGoConfig{Name: "Test"})

		result := flux.AddRedis(RedisOptions{
			Options: redis.Options{Addr: "localhost:6379"},
		})

		assert.NotNil(t, result)
	})
}

func TestRedis_Publish(t *testing.T) {
	t.Run("Should deliver the payload to a subscriber", func(t *testing.T) {
		r, _ := newTestRedis(t)
		ctx := context.Background()

		messages, unsubscribe, err := r.Subscribe(ctx, "topic")
		require.NoError(t, err)
		defer unsubscribe()

		require.NoError(t, r.Publish(ctx, "topic", []byte("hello")))

		select {
		case msg := <-messages:
			assert.Equal(t, "hello", string(msg))
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for the message")
		}
	})
}

func TestRedis_Subscribe(t *testing.T) {
	t.Run("Should close the channel once unsubscribed", func(t *testing.T) {
		r, _ := newTestRedis(t)

		messages, unsubscribe, err := r.Subscribe(context.Background(), "topic")
		require.NoError(t, err)

		unsubscribe()

		_, open := <-messages
		assert.False(t, open, "channel should be closed")
	})

	t.Run("Should tolerate unsubscribing more than once", func(t *testing.T) {
		r, _ := newTestRedis(t)

		_, unsubscribe, err := r.Subscribe(context.Background(), "topic")
		require.NoError(t, err)

		unsubscribe()
		assert.NotPanics(t, unsubscribe)
	})

	t.Run("Should survive the caller context being cancelled", func(t *testing.T) {
		r, _ := newTestRedis(t)

		// Imita o ctx de um OnStart do fx: cancelado logo após a inscrição.
		callerCtx, cancelCaller := context.WithCancel(context.Background())

		messages, unsubscribe, err := r.Subscribe(callerCtx, "topic")
		require.NoError(t, err)
		defer unsubscribe()

		cancelCaller()
		time.Sleep(100 * time.Millisecond)

		require.NoError(t, r.Publish(context.Background(), "topic", []byte("still alive")))

		select {
		case msg, open := <-messages:
			require.True(t, open, "subscription died with the caller context")
			assert.Equal(t, "still alive", string(msg))
		case <-time.After(2 * time.Second):
			t.Fatal("subscription stopped delivering after the caller context was cancelled")
		}
	})

	t.Run("Should return an error when the server is unreachable", func(t *testing.T) {
		r, srv := newTestRedis(t)
		srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		messages, unsubscribe, err := r.Subscribe(ctx, "topic")

		assert.Error(t, err)
		assert.Nil(t, messages)
		assert.Nil(t, unsubscribe)
	})

	t.Run("Should not leak the wait group when the handshake fails", func(t *testing.T) {
		r, srv := newTestRedis(t)
		srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, _, err := r.Subscribe(ctx, "topic")
		require.Error(t, err)

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer stopCancel()

		assert.NoError(t, r.stopSubscriptions(stopCtx))
	})

	t.Run("Should refuse new subscriptions after shutdown", func(t *testing.T) {
		r, _ := newTestRedis(t)

		require.NoError(t, r.stopSubscriptions(context.Background()))

		_, _, err := r.Subscribe(context.Background(), "topic")
		assert.EqualError(t, err, "redis: client is shutting down")
	})
}

func TestRedis_StopSubscriptions(t *testing.T) {
	t.Run("Should drain every open subscription", func(t *testing.T) {
		r, _ := newTestRedis(t)

		channels := make([]<-chan []byte, 0, 5)
		for range 5 {
			messages, _, err := r.Subscribe(context.Background(), "topic")
			require.NoError(t, err)
			channels = append(channels, messages)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		require.NoError(t, r.stopSubscriptions(ctx))

		for i, messages := range channels {
			_, open := <-messages
			assert.Falsef(t, open, "channel %d still open after shutdown", i)
		}
	})

	t.Run("Should give up when the context expires", func(t *testing.T) {
		r, _ := newTestRedis(t)

		_, _, err := r.Subscribe(context.Background(), "topic")
		require.NoError(t, err)

		// Segura o wait group para simular uma inscrição travada.
		r.wg.Add(1)
		defer r.wg.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		start := time.Now()
		err = r.stopSubscriptions(ctx)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 2*time.Second, "should not have waited past the deadline")
	})
}
