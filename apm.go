package fluxgo

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

type Apm struct {
	TraceProvider *sdktrace.TracerProvider
	Tracer        *trace.Tracer
}

func (f *FluxGo) AddApm() *FluxGo {
	f.AddDependency(func() *Apm {
		apm := Apm{}

		return &apm
	})
	f.AddInvoke(func(lc fx.Lifecycle, apm *Apm, o *Otel) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				var traceExporter sdktrace.SpanExporter
				if o.grpcConnection != nil {
					traceExporter, _ = otlptracegrpc.New(context.Background(), otlptracegrpc.WithGRPCConn(o.grpcConnection))
				} else {
					traceExporter, _ = stdouttrace.New()
				}

				traceProvider := sdktrace.NewTracerProvider(
					sdktrace.WithBatcher(traceExporter),
					sdktrace.WithResource(o.res),
				)
				otel.SetTracerProvider(traceProvider)
				otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
					propagation.TraceContext{},
					propagation.Baggage{},
				))

				tracer := traceProvider.Tracer(f.GetCleanName())

				apm.TraceProvider = traceProvider
				apm.Tracer = &tracer

				f.Log("APM", "Started")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := apm.TraceProvider.Shutdown(ctx); err != nil {
					return err
				}
				f.Log("APM", "Stopped")
				return nil
			},
		})
		return nil
	})

	return f
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

type otelTransport struct {
	base http.RoundTripper
}

func (t *otelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if ctx := req.Context(); trace.SpanFromContext(ctx).SpanContext().IsValid() {
		req = req.Clone(ctx)
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	}
	return t.base.RoundTrip(req)
}

// NewHttpClient returns an http.Client that injects W3C traceparent headers
// into outbound requests, enabling trace propagation to downstream services.
func (apm Apm) NewHttpClient() *http.Client {
	return &http.Client{
		Transport: &otelTransport{base: http.DefaultTransport},
	}
}
