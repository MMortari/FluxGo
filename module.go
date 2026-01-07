package fluxgo

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

type FluxModule struct {
	Name string

	dependencies []fx.Option
	invokes      []fx.Option
}

func Module(name string) *FluxModule {
	return &FluxModule{name, make([]fx.Option, 0), make([]fx.Option, 0)}
}

func (f *FluxModule) toFx() fx.Option {
	full := append(f.dependencies, f.invokes...)

	return fx.Module(f.Name, full...)
}

func (f *FluxModule) AddHandler(constructors ...interface{}) *FluxModule {
	f.dependencies = append(f.dependencies, fx.Provide(constructors...))

	return f
}

type RouteIncome struct {
	Entity          EntityData
	FromBody        bool
	FromHeader      bool
	FromQuery       bool
	FromParam       bool
	Validate        bool
	Cache           ICache
	CacheTTL        time.Duration
	CacheInvalidate []string
}
type EntityData any

type HttpHandler func(c *fiber.Ctx, income interface{}) (*GlobalResponse, *GlobalError)
type CronHandler func(ctx context.Context) error

type RouteFn interface{}

func (f *FluxModule) AddRoute(fn RouteFn) *FluxModule {
	f.invokes = append(f.invokes, fx.Invoke(fn))

	return f
}
func (m *FluxModule) HttpRoute(f *FluxGo, group string, method string, path string, config RouteIncome, handler HttpHandler) error {
	http := f.GetHttp()
	r := http.GetRouter(group)

	fun := func(c *fiber.Ctx) error {
		ctx := c.UserContext()

		if cacheRes := config.cache(ctx, f, config, config.cacheKey(c, f.GetCleanName())); cacheRes != nil {
			return c.Status(200).Send([]byte(*cacheRes))
		}

		income, err := config.Parse(http, c)
		if err != nil {
			return c.Status(err.Status).JSON(err)
		}

		res, gErr := handler(c, income)
		if gErr != nil {
			return c.Status(gErr.Status).JSON(gErr)
		}

		go config.cacheStore(ctx, f, config, config.cacheKey(c, f.GetCleanName()), res)
		go config.cacheInvalidate(ctx, f, config)

		if res != nil {
			return c.Status(res.Status).JSON(res.Content)
		}

		return nil
	}

	if r == nil {
		http.app.Add(method, fmt.Sprintf("%s%s", group, path), fun)

		return nil
	}

	(*r).Add(method, path, fun)

	return nil
}
func (m *FluxModule) CronRoute(cron *Cron, crontab string, handler CronHandler) error {
	return cron.Register(crontab, handler)
}
func (m *FluxModule) ToolRoute(f *FluxGo, tools *Tools, handler ToolsInterface) error {
	tools.AddTool(handler)
	return nil
}

func (i *RouteIncome) Parse(http *Http, c *fiber.Ctx) (EntityData, *GlobalError) {
	if i.Entity == nil {
		return nil, nil
	}

	ptr := reflect.New(reflect.TypeOf(i.Entity))
	data := ptr.Interface()

	if i.FromBody {
		if err := c.BodyParser(data); err != nil {
			return nil, &GlobalError{
				Message: "Error parsing JSON",
				Code:    "error.internal",
				Success: false,
				Status:  fiber.StatusBadRequest,
			}
		}
	}
	if i.FromQuery {
		if err := c.QueryParser(data); err != nil {
			return nil, &GlobalError{
				Message: "Error parsing query",
				Code:    "error.internal",
				Success: false,
				Status:  fiber.StatusBadRequest,
			}
		}
	}
	if i.FromParam {
		if err := c.ParamsParser(data); err != nil {
			return nil, &GlobalError{
				Message: "Error parsing params",
				Code:    "error.internal",
				Success: false,
				Status:  fiber.StatusBadRequest,
			}
		}
	}
	if i.FromHeader {
		if err := c.ReqHeaderParser(data); err != nil {
			return nil, &GlobalError{
				Message: "Error parsing headers",
				Code:    "error.internal",
				Success: false,
				Status:  fiber.StatusBadRequest,
			}
		}
	}
	if i.Validate {
		if hasErrors, erros := http.GetValidator().Run(data); hasErrors {
			return nil, erros
		}
	}
	return data, nil
}
func (i *RouteIncome) cacheKey(c *fiber.Ctx, serviceName string) string {
	return i.cacheVal(serviceName, c.OriginalURL())
}
func (i *RouteIncome) cacheVal(serviceName, val string) string {
	return fmt.Sprintf("%s:endpoint:%s", serviceName, val)
}
func (i *RouteIncome) cache(ctx context.Context, f *FluxGo, cache RouteIncome, key string) *string {
	if cache.Cache != nil && cache.CacheTTL.Milliseconds() != 0 {
		ctx, span := f.apm.StartSpan(ctx, "cache/get")
		defer span.End()

		return cache.Cache.Get(ctx, key)
	}

	return nil
}
func (i *RouteIncome) cacheStore(pCtx context.Context, f *FluxGo, cache RouteIncome, key string, res *GlobalResponse) {
	if cache.Cache != nil && cache.CacheTTL.Milliseconds() != 0 {
		ctx, span := f.apm.StartSpan(context.Background(), "cache/store")
		defer span.End()
		span.AddLink(trace.LinkFromContext(pCtx))

		if err := cache.Cache.Store(ctx, key, res.Content, cache.CacheTTL); err != nil {
			span.SetError(err)
		}
	}
}
func (i *RouteIncome) cacheInvalidate(pCtx context.Context, f *FluxGo, cache RouteIncome) {
	if len(cache.CacheInvalidate) == 0 {
		return
	}

	ctx, span := f.apm.StartSpan(context.Background(), "cache/invalidate")
	defer span.End()
	span.AddLink(trace.LinkFromContext(pCtx))

	newKeys := make([]string, 0, len(cache.CacheInvalidate))

	for _, key := range cache.CacheInvalidate {
		newKeys = append(newKeys, fmt.Sprintf("%s*", i.cacheVal(f.GetCleanName(), key)))
	}

	if cache.Cache != nil {
		if err := cache.Cache.Invalidate(ctx, newKeys); err != nil {
			span.SetError(err)
		}
	}
}
