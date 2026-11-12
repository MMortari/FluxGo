package fluxgo

import (
	"strings"
	"testing"

	"go.uber.org/fx"
)

func buildRegistry(flux *FluxGo) (*HttpRegistry, error) {
	var registry *HttpRegistry

	opts := append(flux.GetFxConfig(), fx.Invoke(func(r *HttpRegistry) {
		registry = r
	}), fx.NopLogger)

	if err := fx.New(opts...).Err(); err != nil {
		return nil, err
	}

	return registry, nil
}

func TestHttpRegistry(t *testing.T) {
	noop := func(data HttpConfigData) {}

	t.Run("should register several servers and keep the first as default", func(t *testing.T) {
		flux := New(FluxGoConfig{Name: "Test"})
		flux.AddHttp(HttpOptions{Port: 13333}, noop)
		flux.AddHttp(HttpOptions{Name: "admin", Port: 13334}, noop)

		registry, err := buildRegistry(flux)
		if err != nil {
			t.Fatal(err)
		}

		if len(registry.All()) != 2 {
			t.Fatalf("expected 2 servers, got %d", len(registry.All()))
		}
		if registry.Default() != registry.Get("") {
			t.Error("empty name must resolve to the default server")
		}
		if registry.Get("").GetName() != DefaultServerName {
			t.Errorf("expected default server name %q, got %q", DefaultServerName, registry.Get("").GetName())
		}
		if registry.Get("admin").GetName() != "admin" {
			t.Error("expected admin server to be registered")
		}
		if registry.Get("missing") != nil {
			t.Error("unknown server must resolve to nil")
		}
	})

	t.Run("should reject duplicated server names", func(t *testing.T) {
		flux := New(FluxGoConfig{Name: "Test"})
		flux.AddHttp(HttpOptions{Name: "admin", Port: 13333}, noop)
		flux.AddHttp(HttpOptions{Name: "admin", Port: 13334}, noop)

		_, err := buildRegistry(flux)
		if err == nil || !strings.Contains(err.Error(), `http server "admin" already registered`) {
			t.Errorf("expected duplicated name error, got %v", err)
		}
	})

	t.Run("should reject duplicated ports", func(t *testing.T) {
		flux := New(FluxGoConfig{Name: "Test"})
		flux.AddHttp(HttpOptions{Port: 13333}, noop)
		flux.AddHttp(HttpOptions{Name: "admin", Port: 13333}, noop)

		_, err := buildRegistry(flux)
		if err == nil || !strings.Contains(err.Error(), "http port 13333 already used") {
			t.Errorf("expected duplicated port error, got %v", err)
		}
	})
}
