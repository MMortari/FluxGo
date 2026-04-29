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
		reg, "http", "", f.cleanName, keys,
		&sync.RWMutex{},
		make(map[string]*prometheus.CounterVec), make(map[string]*prometheus.HistogramVec), make(map[string]*prometheus.GaugeVec),
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

func (p *Prometheus) Middleware(app *fiber.App, route string) func(*fiber.Ctx) error {
	prometheus := fiberprometheus.NewWithRegistry(p.reg, p.serviceName, p.namespace, p.subsystem, p.keys)

	prometheus.RegisterAt(app, route)
	prometheus.SetSkipPaths([]string{"/live", "/health", "/readyz"})

	return prometheus.Middleware
}
