package fluxgo

import (
	"context"
	"fmt"
	"log"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"go.uber.org/fx"
)

func (f *FluxGo) AddHttp(http *Http) *FluxGo {
	f.AddDependency(http.Provider(f.apm != nil))
	f.AddInvoke(func(lc fx.Lifecycle) error {
		lc.Append(fx.Hook{
			OnStart: http.Start,
			OnStop:  http.Stop,
		})

		return nil
	})

	f.http = http

	return f
}
func (f *FluxGo) GetHttp() *Http {
	return f.http
}

type Validator struct {
	*validator.Validate
}

type Http struct {
	port      int
	app       *fiber.App
	routers   map[string]*fiber.Router
	validator *Validator
}

type HttpOptions struct {
	Port       int
	ConfigApp  []func(*fiber.App)
	LogRequest bool

	FiberConfig fiber.Config
}

func NewHttp(opt HttpOptions) *Http {
	app := fiber.New(opt.FiberConfig)
	app.Use(helmet.New())
	app.Use(cors.New())

	if opt.LogRequest {
		app.Use(logger.New(logger.Config{
			Format: "${time} ${status} - ${method} ${path} ${latency}\n",
		}))
	}

	for _, fn := range opt.ConfigApp {
		fn(app)
	}

	http := &Http{app: app, port: opt.Port, routers: make(map[string]*fiber.Router)}

	http.GetValidator()

	return http
}

func (h *Http) Start(ctx context.Context) error {
	go func() {
		if err := h.app.Listen(fmt.Sprintf(":%d", h.port)); err != nil {
			log.Panic(err)
		}
	}()
	return nil
}
func (h *Http) Stop(ctx context.Context) error {
	return h.app.Shutdown()
}
func (h *Http) Provider(hasApm bool) interface{} {
	if hasApm {
		return func(apm *TApm) *Http {
			h.GetApp().Use(apm.SetFiberMiddleware())
			return h
		}
	}

	return func() *Http {
		return h
	}
}

func (h *Http) GetApp() *fiber.App {
	return h.app
}
func (h *Http) CreateRouter(prefix string, handlers ...fiber.Handler) *Http {
	route := h.GetApp().Group(prefix, handlers...)
	h.routers[prefix] = &route

	return h
}
func (h *Http) GetRouter(prefix string) fiber.Router {
	route, exists := h.routers[prefix]
	if !exists {
		return nil
	}
	return *route
}

func (h *Http) GetValidator() *Validator {
	if h.validator != nil {
		return h.validator
	}

	var validate = validator.New()
	h.validator = &Validator{validate}

	return h.validator
}
func (v *Validator) Run(data interface{}) (bool, *GlobalError) {
	validate := v.Validate

	validationErrors := []errorResponse{}

	errs := validate.Struct(data)

	if errs != nil {
		for _, err := range errs.(validator.ValidationErrors) {
			var elem errorResponse

			elem.FailedField = err.Field()
			elem.Tag = err.Tag()
			elem.Value = err.Value()

			validationErrors = append(validationErrors, elem)
		}
	}

	hasError := len(validationErrors) > 0

	errors := &GlobalError{
		Code:    "error.validation",
		Success: false,
		Errors:  validationErrors,
	}

	return hasError, errors
}

type errorResponse struct {
	FailedField string      `json:"failed_field"`
	Tag         string      `json:"tag"`
	Value       interface{} `json:"value"`
}

type GlobalResponse[T any] struct {
	Status  int
	Content T
}
type GlobalError struct {
	Message     string `json:"message,omitempty"`
	Code        string `json:"code"`
	Status      int    `json:"-"`
	Success     bool   `json:"success"`
	Errors      any    `json:"errors,omitempty"`
	UserMessage string `json:"user_message,omitempty"`
}
