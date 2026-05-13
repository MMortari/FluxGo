package fluxgo

import (
	"context"

	"go.uber.org/fx"
)

// RouteDefinition represents a route that can be registered on a FluxModule.
// Each definition knows how to produce an fx.Option for dependency injection.
type RouteDefinition interface {
	toFxOption(m *FluxModule) fx.Option
}

// --- HTTP Routes ---

type httpRouteDef struct {
	group  string
	method string
	path   string
	config RouteIncome
	makeFn func(m *FluxModule) interface{}
}

func (d *httpRouteDef) toFxOption(m *FluxModule) fx.Option {
	return fx.Invoke(d.makeFn(m))
}

// HttpDef creates an HTTP route definition that auto-resolves handler *T from DI.
// T is the concrete handler type; PT is the pointer type that implements HttpHandlers.
// If CacheTTL is set and Cache is nil, Redis is auto-injected as the cache.
//
// Usage: HttpDef[MyHandler](group, method, path, config)
func HttpDef[T any, PT interface {
	*T
	HttpHandlers
}](group, method, path string, config RouteIncome) RouteDefinition {
	needsCache := config.CacheTTL > 0 && config.Cache == nil

	return &httpRouteDef{
		group: group, method: method, path: path, config: config,
		makeFn: func(m *FluxModule) interface{} {
			if needsCache {
				return func(f *FluxGo, http *Http, redis *Redis, handler PT) error {
					cfg := config
					cfg.Cache = redis
					return m.HttpRoute(f, http, group, method, path, cfg, handler.HandleHttp)
				}
			}
			return func(f *FluxGo, http *Http, handler PT) error {
				return m.HttpRoute(f, http, group, method, path, config, handler.HandleHttp)
			}
		},
	}
}

// GET creates an HTTP GET route definition.
func GET[T any, PT interface {
	*T
	HttpHandlers
}](group, path string, config RouteIncome) RouteDefinition {
	return HttpDef[T, PT](group, "GET", path, config)
}

// POST creates an HTTP POST route definition.
func POST[T any, PT interface {
	*T
	HttpHandlers
}](group, path string, config RouteIncome) RouteDefinition {
	return HttpDef[T, PT](group, "POST", path, config)
}

// PUT creates an HTTP PUT route definition.
func PUT[T any, PT interface {
	*T
	HttpHandlers
}](group, path string, config RouteIncome) RouteDefinition {
	return HttpDef[T, PT](group, "PUT", path, config)
}

// PATCH creates an HTTP PATCH route definition.
func PATCH[T any, PT interface {
	*T
	HttpHandlers
}](group, path string, config RouteIncome) RouteDefinition {
	return HttpDef[T, PT](group, "PATCH", path, config)
}

// DELETE creates an HTTP DELETE route definition.
func DELETE[T any, PT interface {
	*T
	HttpHandlers
}](group, path string, config RouteIncome) RouteDefinition {
	return HttpDef[T, PT](group, "DELETE", path, config)
}

// --- Cron Routes ---

type cronRouteDef struct {
	makeFn func(m *FluxModule) interface{}
}

func (d *cronRouteDef) toFxOption(m *FluxModule) fx.Option {
	return fx.Invoke(d.makeFn(m))
}

// CronDef creates a cron route that calls handler T's HandleCron method.
// T must implement CronHandlerInterface (use pointer type).
type CronHandlerInterface interface {
	HandleCron(ctx context.Context) error
}

func CronDef[T any, PT interface {
	*T
	CronHandlerInterface
}](crontab string) RouteDefinition {
	return &cronRouteDef{
		makeFn: func(m *FluxModule) interface{} {
			return func(cron *Cron, handler PT) error {
				return m.CronRoute(cron, crontab, handler.HandleCron)
			}
		},
	}
}

// CronFn creates a cron route with an inline function.
// The function receives dependencies from fx and must return a CronHandler.
func CronFn(invokeFn interface{}) RouteDefinition {
	return &cronRouteDef{
		makeFn: func(m *FluxModule) interface{} {
			return invokeFn
		},
	}
}

// --- Kafka/Topic Routes ---

type topicRouteDef struct {
	makeFn func(m *FluxModule) interface{}
}

func (d *topicRouteDef) toFxOption(m *FluxModule) fx.Option {
	return fx.Invoke(d.makeFn(m))
}

// TopicDef creates a Kafka consumer route that calls handler T's HandleMessage method.
// T is the concrete type; PT is the pointer type that implements ConsumerInterface.
func TopicDef[T any, PT interface {
	*T
	ConsumerInterface
}](topic string) RouteDefinition {
	return &topicRouteDef{
		makeFn: func(m *FluxModule) interface{} {
			return func(kafka *Kafka, handler PT) error {
				return m.TopicConsume(kafka, topic, handler.HandleMessage)
			}
		},
	}
}

// --- Tool Routes ---

type toolRouteDef struct {
	makeFn func(m *FluxModule) interface{}
}

func (d *toolRouteDef) toFxOption(m *FluxModule) fx.Option {
	return fx.Invoke(d.makeFn(m))
}

// ToolDef creates a tool route that registers handler T as a tool.
// T must implement ToolsInterface (use pointer type).
// func ToolDef[T ToolsInterface](handler T) RouteDefinition {
func ToolDef[T any, PT interface {
	*T
	ToolsInterface
}]() RouteDefinition {
	return &toolRouteDef{
		makeFn: func(m *FluxModule) interface{} {
			return func(f *FluxGo, tools *Tools, handler PT) error {
				return m.ToolRoute(f, tools, handler)
			}
		},
	}
}
