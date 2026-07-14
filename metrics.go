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

type Metrics struct {
	Provider          *sdkmetric.MeterProvider
	meter             metric.Meter
	counterIntMap     map[string]metric.Int64Counter
	counterFloatMap   map[string]metric.Float64Counter
	histogramIntMap   map[string]metric.Int64Histogram
	histogramFloatMap map[string]metric.Float64Histogram
	gaugeIntMap       map[string]metric.Int64Gauge
	gaugeFloatMap     map[string]metric.Float64Gauge
	mutex             sync.RWMutex
}

func (f *FluxGo) AddMetrics() *FluxGo {
	f.AddDependency(func() *Metrics {
		return &Metrics{
			counterIntMap:     make(map[string]metric.Int64Counter),
			counterFloatMap:   make(map[string]metric.Float64Counter),
			histogramIntMap:   make(map[string]metric.Int64Histogram),
			histogramFloatMap: make(map[string]metric.Float64Histogram),
			gaugeIntMap:       make(map[string]metric.Int64Gauge),
			gaugeFloatMap:     make(map[string]metric.Float64Gauge),
			mutex:             sync.RWMutex{},
		}
	})
	f.AddInvoke(func(lc fx.Lifecycle, m *Metrics, o *Otel) error {
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
				if m.Provider != nil {
					if err := m.Provider.Shutdown(ctx); err != nil {
						return err
					}
				}
				f.Log("METRICS", "Stopped")
				return nil
			},
		})
		return nil
	})

	return f
}

func (m *Metrics) NewIntCounter(name, description string) metric.Int64Counter {
	counter, err := m.meter.Int64Counter(name, metric.WithDescription(description))
	if err != nil {
		panic(err)
	}

	m.mutex.Lock()
	m.counterIntMap[name] = counter
	m.mutex.Unlock()

	return counter
}
func (m *Metrics) NewFloatCounter(name, description string) metric.Float64Counter {
	counter, err := m.meter.Float64Counter(name, metric.WithDescription(description))
	if err != nil {
		panic(err)
	}

	m.mutex.Lock()
	m.counterFloatMap[name] = counter
	m.mutex.Unlock()

	return counter
}

func (m *Metrics) NewIntHistogram(name, description string, boundaries []float64) metric.Int64Histogram {
	hist, err := m.meter.Int64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithExplicitBucketBoundaries(boundaries...),
	)
	if err != nil {
		panic(err)
	}

	m.mutex.Lock()
	m.histogramIntMap[name] = hist
	m.mutex.Unlock()

	return hist
}
func (m *Metrics) NewFloatHistogram(name, description string, boundaries []float64) metric.Float64Histogram {
	hist, err := m.meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithExplicitBucketBoundaries(boundaries...),
	)
	if err != nil {
		panic(err)
	}

	m.mutex.Lock()
	m.histogramFloatMap[name] = hist
	m.mutex.Unlock()

	return hist
}

func (m *Metrics) NewIntGauge(name, description string) metric.Int64Gauge {
	gauge, err := m.meter.Int64Gauge(name, metric.WithDescription(description))
	if err != nil {
		panic(err)
	}

	m.mutex.Lock()
	m.gaugeIntMap[name] = gauge
	m.mutex.Unlock()

	return gauge
}
func (m *Metrics) NewFloatGauge(name, description string) metric.Float64Gauge {
	gauge, err := m.meter.Float64Gauge(name, metric.WithDescription(description))
	if err != nil {
		panic(err)
	}

	m.mutex.Lock()
	m.gaugeFloatMap[name] = gauge
	m.mutex.Unlock()

	return gauge
}

func (m *Metrics) GetCounterInt(name string) metric.Int64Counter {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.counterIntMap[name]
}
func (m *Metrics) GetCounterFloat(name string) metric.Float64Counter {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.counterFloatMap[name]
}

func (m *Metrics) GetHistogramInt(name string) metric.Int64Histogram {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.histogramIntMap[name]
}
func (m *Metrics) GetHistogramFloat(name string) metric.Float64Histogram {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.histogramFloatMap[name]
}

func (m *Metrics) GetGaugeInt(name string) metric.Int64Gauge {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.gaugeIntMap[name]
}
func (m *Metrics) GetGaugeFloat(name string) metric.Float64Gauge {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.gaugeFloatMap[name]
}
