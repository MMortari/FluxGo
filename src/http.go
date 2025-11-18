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

	f.AddDependency(fx.Provide(func() *fiber.App {
		return app
	}))
	f.AddInvoke(fx.Invoke(func(lc fx.Lifecycle) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				go func() {
					if err := app.Listen(fmt.Sprintf(":%d", opt.Port)); err != nil {
						log.Panic(err)
					}
				}()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				return app.Shutdown()
			},
		})

		return nil
	}))

	return f
}

func httpStart(lc fx.Lifecycle, app *fiber.App) error {
	port := 3333

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := app.Listen(fmt.Sprintf(":%d", port)); err != nil {
					log.Panic(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return app.Shutdown()
		},
	})

	return nil
}
