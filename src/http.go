package fluxgo

import (
	"context"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"go.uber.org/fx"
)

type HttpOptions struct {
	Port      int
	ConfigApp []func(*fiber.App)

	UseApmOnFiber bool
}

func (f *FluxGo) AddHttp(opt HttpOptions) *FluxGo {
	app := fiber.New(fiber.Config{
		AppName: f.Name,
	})
	app.Use(helmet.New())
	app.Use(cors.New())

	for _, fn := range opt.ConfigApp {
		fn(app)
	}

	f.AddDependency(fx.Provide(getProvideFunction(opt, app)))
	f.AddInvoke(fx.Invoke(func(lc fx.Lifecycle, appI *fiber.App) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				go func() {
					if err := appI.Listen(fmt.Sprintf(":%d", opt.Port)); err != nil {
						log.Panic(err)
					}
				}()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				return appI.Shutdown()
			},
		})

		return nil
	}))

	return f
}

func getProvideFunction(opt HttpOptions, app *fiber.App) interface{} {
	if opt.UseApmOnFiber {
		return func(apm *TApm) *fiber.App {
			app.Use(apm.SetFiberMiddleware())
			return app
		}
	}

	return func() *fiber.App {
		return app
	}
}
