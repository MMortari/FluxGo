package fluxgo

import (
	"context"
	"reflect"

	"github.com/gofiber/fiber/v2"
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
	Entity     EntityData
	FromBody   bool
	FromHeader bool
	FromQuery  bool
	FromParam  bool
	Validate   bool
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

	r.Add(method, path, func(c *fiber.Ctx) error {
		income, err := config.Parse(http, c)
		if err != nil {
			return c.Status(err.Status).JSON(err)
		}

		res, gErr := handler(c, income)
		if gErr != nil {
			return c.Status(gErr.Status).JSON(gErr)
		}

		if res != nil {
			return c.Status(res.Status).JSON(res.Content)
		}

		return nil
	})

	return nil
}
func (m *FluxModule) CronRoute(cron *Cron, crontab string, handler CronHandler) error {
	return cron.Register(crontab, handler)
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
