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
	Port               int
	Interceptors       []grpc.UnaryServerInterceptor
	StreamInterceptors []grpc.StreamServerInterceptor
	// Reflection enables gRPC server reflection (useful for grpcurl and Postman).
	Reflection bool
}

// Grpc wraps a gRPC server managed by FluxGo.
type Grpc struct {
	server *grpc.Server
	opts   GrpcOptions
}

// GrpcHandlerInterface must be implemented by handlers registered via GrpcDef.
// RegisterGrpc is called once during startup to register the proto service on the server.
type GrpcHandlerInterface interface {
	RegisterGrpc(server *grpc.Server)
}

// AddGrpc registers a gRPC server with lifecycle management.
// Handlers are registered via GrpcDef in each FluxModule.
func (f *FluxGo) AddGrpc(opts GrpcOptions) *FluxGo {
	f.AddDependency(func() *Grpc {
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

		return &Grpc{server: server, opts: opts}
	})

	f.AddInvoke(func(lc fx.Lifecycle, g *Grpc) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				lis, err := net.Listen("tcp", fmt.Sprintf(":%d", g.opts.Port))
				if err != nil {
					return err
				}

				go func() {
					if err := g.server.Serve(lis); err != nil && err != grpc.ErrServerStopped {
						f.log("GRPC", fmt.Sprintf("Server error: %v", err))
					}
				}()

				f.log("GRPC", fmt.Sprintf("Started on port %d", g.opts.Port))
				return nil
			},
			OnStop: func(ctx context.Context) error {
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

				f.log("GRPC", "Stopped")

				return nil
			},
		})
		return nil
	})

	return f
}
