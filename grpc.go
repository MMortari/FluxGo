package fluxgo

import (
	"context"
	"fmt"
	"net"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// GrpcOptions configures the gRPC server.
type GrpcOptions struct {
	// Name identifies this server when more than one is registered.
	// Leave empty for the default server.
	Name               string
	Port               int
	Interceptors       []grpc.UnaryServerInterceptor
	StreamInterceptors []grpc.StreamServerInterceptor
	// Reflection enables gRPC server reflection (useful for grpcurl and Postman).
	Reflection bool
}

// Grpc wraps a gRPC server managed by FluxGo.
type Grpc struct {
	name    string
	started bool
	server  *grpc.Server
	opts    GrpcOptions
}

// GetName returns the name this server was registered under.
func (g *Grpc) GetName() string {
	return g.name
}

// GetServer returns the underlying gRPC server.
func (g *Grpc) GetServer() *grpc.Server {
	return g.server
}

// GrpcRegistry holds every gRPC server registered through AddGrpc, keyed by name.
type GrpcRegistry struct {
	servers map[string]*Grpc
	order   []*Grpc
	def     *Grpc
}

// Get returns the server registered under name.
// An empty name resolves to the default server (the first one registered).
func (r *GrpcRegistry) Get(name string) *Grpc {
	if name == "" {
		return r.def
	}

	return r.servers[name]
}

// Default returns the first server registered through AddGrpc.
func (r *GrpcRegistry) Default() *Grpc {
	return r.def
}

// All returns every registered server, in registration order.
func (r *GrpcRegistry) All() []*Grpc {
	return r.order
}

// GrpcClientOptions configures an outgoing gRPC client connection.
type GrpcClientOptions struct {
	// Extra dial options appended after the otelgrpc stats handler.
	DialOptions []grpc.DialOption
}

// NewGrpcClient creates a gRPC client connection with OTEL trace propagation enabled.
// Always use this instead of grpc.NewClient/grpc.Dial so the outgoing call carries
// the current trace context and spans are linked across services.
func NewGrpcClient(target string, opts GrpcClientOptions) (*grpc.ClientConn, error) {
	dialOpts := []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}
	dialOpts = append(dialOpts, opts.DialOptions...)
	return grpc.NewClient(target, dialOpts...)
}

// GrpcHandlerInterface must be implemented by handlers registered via GrpcDef.
// RegisterGrpc is called once during startup to register the proto service on the server.
type GrpcHandlerInterface interface {
	RegisterGrpc(server *grpc.Server)
}

// AddGrpc registers a gRPC server with lifecycle management.
// It can be called more than once to expose several servers (each on its own port);
// name them through GrpcOptions.Name and select them via GrpcDef("name").
// The first server registered is the default one and is the one injected as *Grpc.
// Handlers are registered via GrpcDef in each FluxModule.
func (f *FluxGo) AddGrpc(opts GrpcOptions) *FluxGo {
	f.grpcSpecs = append(f.grpcSpecs, opts)

	if f.grpcRegistered {
		return f
	}
	f.grpcRegistered = true

	f.AddDependency(func() (*GrpcRegistry, error) {
		registry := &GrpcRegistry{servers: make(map[string]*Grpc)}
		ports := make(map[int]string)

		for _, opt := range f.grpcSpecs {
			name := opt.Name
			if name == "" {
				name = DefaultServerName
			}

			if _, exists := registry.servers[name]; exists {
				return nil, fmt.Errorf("grpc server %q already registered", name)
			}
			if other, exists := ports[opt.Port]; exists {
				return nil, fmt.Errorf("grpc port %d already used by server %q", opt.Port, other)
			}
			ports[opt.Port] = name

			g := buildGrpc(name, opt)

			registry.servers[name] = g
			registry.order = append(registry.order, g)
			if registry.def == nil {
				registry.def = g
			}
		}

		return registry, nil
	})
	f.AddDependency(func(registry *GrpcRegistry) *Grpc { return registry.def })

	f.AddInvoke(func(lc fx.Lifecycle, registry *GrpcRegistry) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				for _, g := range registry.All() {
					lis, err := net.Listen("tcp", fmt.Sprintf(":%d", g.opts.Port))
					if err != nil {
						return fmt.Errorf("failed to bind gRPC port %d: %w", g.opts.Port, err)
					}
					g.started = true

					go func(g *Grpc) {
						if err := g.server.Serve(lis); err != nil && err != grpc.ErrServerStopped {
							f.Log("GRPC", fmt.Sprintf("[%s] Server error: %v", g.name, err))
						}
					}(g)

					f.Log("GRPC", fmt.Sprintf("[%s] Started on port %d", g.name, g.opts.Port))
				}

				return nil
			},
			OnStop: func(ctx context.Context) error {
				for _, g := range registry.All() {
					if !g.started {
						continue
					}
					g.started = false

					done := make(chan struct{})

					go func() {
						g.server.GracefulStop()
						close(done)
					}()

					select {
					case <-done:
					case <-ctx.Done():
						g.server.Stop()
					}

					f.Log("GRPC", fmt.Sprintf("[%s] Stopped", g.name))
				}

				return nil
			},
		})

		return nil
	})

	return f
}

func buildGrpc(name string, opts GrpcOptions) *Grpc {
	serverOpts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}

	if len(opts.Interceptors) > 0 {
		serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(opts.Interceptors...))
	}
	if len(opts.StreamInterceptors) > 0 {
		serverOpts = append(serverOpts, grpc.ChainStreamInterceptor(opts.StreamInterceptors...))
	}

	server := grpc.NewServer(serverOpts...)

	if opts.Reflection {
		reflection.Register(server)
	}

	return &Grpc{name: name, server: server, opts: opts}
}
