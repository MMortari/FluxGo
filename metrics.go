package fluxgo

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
)

type FluxGoMetrics struct {
	Provider     *sdkmetric.MeterProvider
	meter        metric.Meter
	counterMap   map[string]metric.Int64Counter
	histogramMap map[string]metric.Float64Histogram
	gaugeMap     map[string]metric.Float64Gauge
	mutex        *sync.RWMutex
}

func (f *FluxGo) AddMetrics() *FluxGo {
	f.AddDependency(func() *FluxGoMetrics {
		return &FluxGoMetrics{
			counterMap:   make(map[string]metric.Int64Counter),
			histogramMap: make(map[string]metric.Float64Histogram),
			gaugeMap:     make(map[string]metric.Float64Gauge),
			mutex:        &sync.RWMutex{},
		}
	})
	f.AddInvoke(func(lc fx.Lifecycle, m *FluxGoMetrics, o *Otel) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				var exporter sdkmetric.Exporter
				var err error

				if o.grpcConnection != nil {
					exporter, err = otlpmetricgrpc.New(context.Background(), otlpmetricgrpc.WithGRPCConn(o.grpcConnection))
				} else {
					exporter, err = stdoutmetric.New()
				}
				if err != nil {
					return err
				}

				meterProvider := sdkmetric.NewMeterProvider(
					sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
					sdkmetric.WithResource(o.res),
				)
				otel.SetMeterProvider(meterProvider)

				m.Provider = meterProvider
				m.meter = meterProvider.Meter(f.GetCleanName())

				f.Log("METRICS", "Started")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := m.Provider.Shutdown(ctx); err != nil {
					return err
				}
				f.Log("METRICS", "Stopped")
				return nil
			},
		})
		return nil
	})

	return f
}

func (m *FluxGoMetrics) NewInt64Counter(name, description string) metric.Int64Counter {
	counter, err := m.meter.Int64Counter(name, metric.WithDescription(description))
	if err != nil {
		panic(err)
	}

	m.mutex.Lock()
	m.counterMap[name] = counter
	m.mutex.Unlock()

	return counter
}

func (m *FluxGoMetrics) NewFloat64Histogram(name, description string, boundaries []float64) metric.Float64Histogram {
	hist, err := m.meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithExplicitBucketBoundaries(boundaries...),
	)
	if err != nil {
		panic(err)
	}

	m.mutex.Lock()
	m.histogramMap[name] = hist
	m.mutex.Unlock()

	return hist
}

func (m *FluxGoMetrics) NewFloat64Gauge(name, description string) metric.Float64Gauge {
	gauge, err := m.meter.Float64Gauge(name, metric.WithDescription(description))
	if err != nil {
		panic(err)
	}

	m.mutex.Lock()
	m.gaugeMap[name] = gauge
	m.mutex.Unlock()

	return gauge
}

func (m *FluxGoMetrics) GetCounter(name string) metric.Int64Counter {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.counterMap[name]
}

func (m *FluxGoMetrics) GetHistogram(name string) metric.Float64Histogram {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.histogramMap[name]
}

func (m *FluxGoMetrics) GetGauge(name string) metric.Float64Gauge {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.gaugeMap[name]
}
