package fluxgo

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"go.uber.org/fx"
)

func (f *FluxGo) AddHttp(http *Http) *FluxGo {
	f.AddDependency(func() *Http {
		return http
	})
	f.AddInvoke(func(lc fx.Lifecycle) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				err := http.Start(ctx)
				if err != nil {
					return err
				}
				f.log("HTTP", fmt.Sprintf("Running on port %d", http.port))

				return nil
			},
			OnStop: func(ctx context.Context) error {
				err := http.Stop(ctx)
				if err != nil {
					return err
				}
				f.log("HTTP", "Stopped")

				return nil
			},
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
	ConfigApp  func(*fiber.App)
	LogRequest bool

	FiberConfig fiber.Config
	Apm         *Apm
}

func NewHttp(opt HttpOptions) *Http {
	opt.FiberConfig.DisableStartupMessage = true
	app := fiber.New(opt.FiberConfig)

	if opt.Apm != nil {
		app.Use(opt.Apm.SetFiberMiddleware())
	}
	app.Use(helmet.New())
	app.Use(cors.New())

	if opt.ConfigApp != nil {
		opt.ConfigApp(app)
	}
	if opt.LogRequest {
		app.Use(logger.New(logger.Config{
			Format: "${time} ${status} - ${method} ${path} ${latency}\n",
		}))
	}

	http := &Http{app: app, port: opt.Port, routers: make(map[string]*fiber.Router)}

	http.GetValidator()

	return http
}

func (h *Http) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		addr := fmt.Sprintf(":%d", h.port)

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			errCh <- fmt.Errorf("failed to bind port %d: %w", h.port, err)
			return
		}

		errCh <- nil

		if err := h.app.Listener(listener); err != nil {

			select {
			case <-ctx.Done():

				return
			default:

				log.Printf("Server error: %v", err)
			}
		}
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}

		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}
func (h *Http) Stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- h.app.ShutdownWithContext(shutdownCtx)
	}()

	select {
	case err := <-done:
		return err
	case <-shutdownCtx.Done():
		return fmt.Errorf("shutdown timeout exceeded")
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
func (h *Http) GetRouter(prefix string) *fiber.Router {
	route, exists := h.routers[prefix]
	if !exists {
		return nil
	}
	return route
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
		Status:  400,
	}

	return hasError, errors
}

type errorResponse struct {
	FailedField string      `json:"failed_field"`
	Tag         string      `json:"tag"`
	Value       interface{} `json:"value"`
}

type GlobalResponse struct {
	Status  int
	Content interface{}
}
type GlobalError struct {
	Message     string `json:"message,omitempty"`
	Code        string `json:"code"`
	Status      int    `json:"-"`
	Success     bool   `json:"success"`
	Errors      any    `json:"errors,omitempty"`
	UserMessage string `json:"user_message,omitempty"`
}

func ErrorInternalError(message string) *GlobalError {
	return &GlobalError{
		Message: message,
		Code:    "internal_error",
		Status:  http.StatusInternalServerError,
		Success: false,
	}
}
func ErrorNotFound(message string) *GlobalError {
	return &GlobalError{
		Message: message,
		Code:    "not_found",
		Status:  http.StatusNotFound,
		Success: false,
	}
}
