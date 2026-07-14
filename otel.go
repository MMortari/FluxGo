package fluxgo

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Otel struct {
	grpcConnection *grpc.ClientConn

	res *resource.Resource
	opt OtelOptions
}

type OtelOptions struct {
	CollectorURL string
	Exporter     string
}

func (f *FluxGo) addOtel(opt OtelOptions) *FluxGo {
	res := buildOtelResource(f)

	otel := Otel{res: res, opt: opt}

	if opt.Exporter == "grpc" {
		conn, err := grpc.NewClient(opt.CollectorURL,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			panic(err)
		}
		otel.grpcConnection = conn
	}

	f.otel = &otel

	f.AddDependency(func() *Otel {
		return &otel
	})
	f.AddInvoke(func(lc fx.Lifecycle, o *Otel) error {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				if o.grpcConnection != nil {
					return o.grpcConnection.Close()
				}
				return nil
			},
		})
		return nil
	})

	return f
}

func buildOtelResource(f *FluxGo) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(f.GetCleanName()),
		semconv.ServiceVersionKey.String(f.Version),
		semconv.DeploymentEnvironmentKey.String(f.Env.Env),
		attribute.String("service.language.name", "go"),
		attribute.String("host.name", os.Getenv("POD_NAME")),
	)
}
