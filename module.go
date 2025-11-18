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
	return &FluxModule{Name: name}
}

func (f *FluxModule) toFx() fx.Option {
	full := append(f.dependencies, f.invokes...)

	return fx.Module(f.Name, full...)
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

type RouteHandler func(ctx context.Context, c *fiber.Ctx, income EntityData) error

func (f *FluxModule) HttpRoute(group string, method string, path string, config RouteIncome, handler RouteHandler) *FluxModule {
	f.invokes = append(f.invokes, fx.Invoke(func(http *Http) {
		router := http.GetRouter(group)

		router.Add(method, path, func(c *fiber.Ctx) error {
			data, err := config.Parse(http, c)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(err)
			}
			return handler(c.UserContext(), c, data)
		})
	}))

	return f
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
