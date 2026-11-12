package fluxgo

import (
	"sync"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Prometheus struct {
	reg         *prometheus.Registry
	namespace   string
	subsystem   string
	serviceName string
	keys        map[string]string
	mutex       *sync.RWMutex

	counterVec   map[string]*prometheus.CounterVec
	histogramVec map[string]*prometheus.HistogramVec
	gaugeVec     map[string]*prometheus.GaugeVec

	fiberMetrics *fiberprometheus.FiberPrometheus
}

func (f *FluxGo) AddPrometheus() *Prometheus {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())

	keys := map[string]string{
		"service_name": f.cleanName,
		"version":      f.Version,
		"environment":  f.Env.Env,
	}

	prom := Prometheus{
		reg:          reg,
		namespace:    "http",
		subsystem:    "",
		serviceName:  f.cleanName,
		keys:         keys,
		mutex:        &sync.RWMutex{},
		counterVec:   make(map[string]*prometheus.CounterVec),
		histogramVec: make(map[string]*prometheus.HistogramVec),
		gaugeVec:     make(map[string]*prometheus.GaugeVec),
	}

	f.AddDependency(func() *Prometheus { return &prom })

	return &prom
}

func (f *Prometheus) GetCounterVec(name string) *prometheus.CounterVec {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.counterVec[name]
}
func (f *Prometheus) GetHistogramVec(name string) *prometheus.HistogramVec {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.histogramVec[name]
}
func (f *Prometheus) GetGaugeVec(name string) *prometheus.GaugeVec {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.gaugeVec[name]
}

func (p *Prometheus) NewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec {
	vec := prometheus.NewCounterVec(opts, labelNames)

	p.mutex.Lock()
	p.counterVec[opts.Name] = vec
	p.mutex.Unlock()

	p.reg.MustRegister(vec)

	return vec
}
func (p *Prometheus) NewHistogramVec(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec {
	vec := prometheus.NewHistogramVec(opts, labelNames)

	p.mutex.Lock()
	p.histogramVec[opts.Name] = vec
	p.mutex.Unlock()

	p.reg.MustRegister(vec)

	return vec
}
func (p *Prometheus) NewGaugeVec(opts prometheus.GaugeOpts, labelNames []string) *prometheus.GaugeVec {
	vec := prometheus.NewGaugeVec(opts, labelNames)

	p.mutex.Lock()
	p.gaugeVec[opts.Name] = vec
	p.mutex.Unlock()

	p.reg.MustRegister(vec)

	return vec
}

// Middleware exposes the metrics endpoint on app and returns the collecting middleware.
// The underlying collectors are created once and shared across every app it is called
// with, so several HTTP servers can expose the same registry without registering
// duplicate collectors.
func (p *Prometheus) Middleware(app *fiber.App, route string) func(*fiber.Ctx) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.fiberMetrics == nil {
		p.fiberMetrics = fiberprometheus.NewWithRegistry(p.reg, p.serviceName, p.namespace, p.subsystem, p.keys)
		p.fiberMetrics.SetSkipPaths([]string{"/live", "/health", "/readyz"})
	}

	p.fiberMetrics.RegisterAt(app, route)

	return p.fiberMetrics.Middleware
}
