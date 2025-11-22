package user

import (
	"context"
	"log"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/handlers"
	"github.com/gofiber/fiber/v2"
)

func Module() *fluxgo.FluxModule {
	mod := fluxgo.Module("user")

	mod.AddHandler(handlers.HandlerGetUserStart)
	mod.AddRoute(func(f *fluxgo.FluxGo, handler *handlers.HandlerGetUser) error {
		return mod.HttpRoute(f, "/public", "GET", "/user", fluxgo.RouteIncome{}, func(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
			resp, err := handler.Execute(c.UserContext(), "")
			if err != nil {
				return nil, err
			}
			return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
		})
	})
	mod.AddRoute(func(cron *fluxgo.Cron, logger *fluxgo.Logger, handler *handlers.HandlerGetUser) error {
		return mod.CronRoute(cron, "* * * * *", func(ctx context.Context) error {
			logger.Infoln("Cron executed")
			log.Println("Cron executed")
			return nil
		})
	})

	return mod
}
