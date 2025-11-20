package fluxgo

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

type Apm struct {
	TraceProvider *sdktrace.TracerProvider
	Tracer        *trace.Tracer
	CollectorURL  string
}

type ApmOptions struct {
	CollectorURL string
	Exporter     string
}

func (f *FluxGo) AddApm(opt ApmOptions) *FluxGo {
	tp, tracer := configApm(configApmI{
		ApmOptions:     opt,
		ServiceName:    f.Name,
		ServiceVersion: f.Version,
		Env:            f.Env,
	})

	apm := Apm{
		TraceProvider: tp,
		Tracer:        tracer,
		CollectorURL:  opt.CollectorURL,
	}

	f.apm = &apm

	f.AddDependency(func(lc fx.Lifecycle) *Apm {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return apm.TraceProvider.Shutdown(ctx)
			},
		})

		return &apm
	})

	return f
}
func (f *FluxGo) GetApm() *Apm {
	return f.apm
}

type Tracer = trace.Tracer
type Span struct {
	trace.Span
}

func (span *Span) SetError(err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func (s *Span) SetAttributeString(key, val string) {
	s.SetAttributes(attribute.String(key, val))
}
func (s *Span) SetAttributeNumber(key string, val string) {
	s.SetAttributes(attribute.String(key, val))
}

type configApmI struct {
	ApmOptions
	ServiceName    string
	ServiceVersion string
	Env            string
}

func configApm(config configApmI) (*sdktrace.TracerProvider, *Tracer) {
	exporter, err := getExporter(config)
	if err != nil {
		log.Fatal(err)
	}

	if exporter == nil {
		log.Fatal("No exporter provided")
		return nil, nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(config.ServiceName),
				semconv.ServiceVersionKey.String(config.ServiceVersion),
				semconv.DeploymentEnvironmentKey.String(config.Env),
				attribute.String("service.language.name", "go"),
				attribute.String("host.name", os.Getenv("POD_NAME")),
			)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	tracer := tp.Tracer(config.ServiceName)

	return tp, &tracer
}
func (apm Apm) ShutdownApm() {
	if apm.TraceProvider == nil {
		return
	}

	if err := apm.TraceProvider.Shutdown(context.Background()); err != nil {
		log.Printf("Error shutting down tracer provider: %v\n", err)
	}
}

func (apm Apm) SetFiberMiddleware() func(*fiber.Ctx) error {
	return otelfiber.Middleware(otelfiber.WithSpanNameFormatter(func(ctx *fiber.Ctx) string {
		return fmt.Sprintf("%s %s", ctx.Method(), ctx.Route().Path)
	}))
}

func (apm Apm) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, Span) {
	if apm.Tracer == nil {
		return ctx, Span{trace.SpanFromContext(ctx)}
	}

	ctx, span := (*apm.Tracer).Start(ctx, name, opts...)

	return ctx, Span{span}
}
func (apm Apm) GetSpanFromContext(ctx context.Context) Span {
	return Span{trace.SpanFromContext(ctx)}
}

func SetAttributes(attributes ...attribute.KeyValue) trace.SpanStartEventOption {
	return trace.WithAttributes(attributes...)
}

func getExporter(config configApmI) (sdktrace.SpanExporter, error) {
	if config.CollectorURL != "" && config.Exporter == "grpc" {
		return otlptrace.New(
			context.Background(),
			otlptracegrpc.NewClient(
				otlptracegrpc.WithInsecure(),
				otlptracegrpc.WithEndpoint(config.CollectorURL),
			),
		)
	} else if config.Exporter == "log" {
		return stdouttrace.New()
	}

	panic("Invalid APM exporter type")
}
