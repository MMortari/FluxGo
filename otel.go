package fluxgo

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Otel struct {
	traceProvider *sdktrace.TracerProvider
	logProvider   *sdklog.LoggerProvider
	Tracer        *trace.Tracer

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

	otel := Otel{
		res: res,
		opt: opt,
	}

	f.otel = &otel

	f.AddDependency(func() *Otel {
		return &otel
	})
	f.AddInvoke(func(lc fx.Lifecycle, o *Otel) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				conn, err := grpc.NewClient("otel-collector:4317",
					grpc.WithTransportCredentials(insecure.NewCredentials()),
				)
				if err != nil {
					return err
				}

				o.grpcConnection = conn

				return nil
			},
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
