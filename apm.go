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
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	logGlobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

type Apm struct {
	TraceProvider *sdktrace.TracerProvider
	LogProvider   *sdklog.LoggerProvider
	Tracer        *trace.Tracer
	CollectorURL  string
}

type ApmOptions struct {
	CollectorURL string
	Exporter     string
}

func (f *FluxGo) AddApm(opt ApmOptions) *FluxGo {
	res := buildApmResource(configApmI{
		ApmOptions:     opt,
		ServiceName:    f.GetCleanName(),
		ServiceVersion: f.Version,
		Env:            f.Env.Env,
	})

	tp, tracer := configTraceProvider(configApmI{
		ApmOptions:     opt,
		ServiceName:    f.GetCleanName(),
		ServiceVersion: f.Version,
		Env:            f.Env.Env,
	}, res)

	lp := configLogProvider(configApmI{
		ApmOptions:     opt,
		ServiceName:    f.GetCleanName(),
		ServiceVersion: f.Version,
		Env:            f.Env.Env,
	}, res)

	apm := Apm{
		TraceProvider: tp,
		LogProvider:   lp,
		Tracer:        tracer,
		CollectorURL:  opt.CollectorURL,
	}

	f.apm = &apm

	// Re-configure logger to wire in the otelslog bridge now that LogProvider is ready.
	if f.logger != nil {
		f.ConfigLogger(f.logger.opt)
	}

	f.AddDependency(func() *Apm {
		return &apm
	})
	f.AddInvoke(func(lc fx.Lifecycle, apm *Apm) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				f.log("APM", "Started")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := apm.TraceProvider.Shutdown(ctx); err != nil {
					return err
				}
				if err := apm.LogProvider.Shutdown(ctx); err != nil {
					return err
				}
				f.log("APM", "Stopped")
				return nil
			},
		})
		return nil
	})

	return f
}

func (f *FluxGo) GetApm() *Apm {
	if f.apm == nil {
		log.Fatal("APM not initialized. Please call AddApm() before using GetApm().")
	}
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

type configApmI struct {
	ApmOptions
	ServiceName    string
	ServiceVersion string
	Env            string
}

func buildApmResource(config configApmI) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(config.ServiceName),
		semconv.ServiceVersionKey.String(config.ServiceVersion),
		semconv.DeploymentEnvironmentKey.String(config.Env),
		attribute.String("service.language.name", "go"),
		attribute.String("host.name", os.Getenv("POD_NAME")),
	)
}

func configTraceProvider(config configApmI, res *resource.Resource) (*sdktrace.TracerProvider, *Tracer) {
	exporter, err := getTraceExporter(config)
	if err != nil {
		log.Fatal(err)
	}
	if exporter == nil {
		log.Fatal("No trace exporter provided")
		return nil, nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
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

func configLogProvider(config configApmI, res *resource.Resource) *sdklog.LoggerProvider {
	exporter, err := getLogExporter(config)
	if err != nil {
		log.Fatal(err)
	}
	if exporter == nil {
		log.Fatal("No log exporter provided")
		return nil
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)

	logGlobal.SetLoggerProvider(lp)
	return lp
}

func (apm Apm) ShutdownApm() {
	if apm.TraceProvider != nil {
		if err := apm.TraceProvider.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v\n", err)
		}
	}
	if apm.LogProvider != nil {
		if err := apm.LogProvider.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down log provider: %v\n", err)
		}
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

func getTraceExporter(config configApmI) (sdktrace.SpanExporter, error) {
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

func getLogExporter(config configApmI) (sdklog.Exporter, error) {
	if config.CollectorURL != "" && config.Exporter == "grpc" {
		return otlploggrpc.New(
			context.Background(),
			otlploggrpc.WithInsecure(),
			otlploggrpc.WithEndpoint(config.CollectorURL),
		)
	} else if config.Exporter == "log" {
		return stdoutlog.New()
	}

	panic("Invalid APM log exporter type")
}
