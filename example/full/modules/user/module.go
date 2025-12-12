package user

import (
	"context"
	"log"
	"time"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/handlers"
	"github.com/gofiber/fiber/v2"
)

func Module() *fluxgo.FluxModule {
	mod := fluxgo.Module("user")

	mod.AddHandler(handlers.HandlerGetUserStart)

	mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
		return mod.HttpRoute(f, "/public", "GET", "/user", fluxgo.RouteIncome{
			Cache:    redis,
			CacheTTL: time.Hour,
		}, func(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
			resp, err := handler.Execute(c.UserContext(), "")
			if err != nil {
				return nil, err
			}
			return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
		})
	})
	mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
		return mod.HttpRoute(f, "/public", "GET", "/user/:id_user", fluxgo.RouteIncome{
			Cache:    redis,
			CacheTTL: time.Hour,
		}, func(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
			resp, err := handler.Execute(c.UserContext(), "")
			if err != nil {
				return nil, err
			}
			return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
		})
	})
	mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
		return mod.HttpRoute(f, "/public", "POST", "/refresh", fluxgo.RouteIncome{
			Cache:           redis,
			CacheInvalidate: []string{"/public/user"},
		}, func(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
			resp, err := handler.Execute(c.UserContext(), "")
			if err != nil {
				return nil, err
			}
			return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
		})
	})
	mod.AddRoute(func(cron *fluxgo.Cron, kafka *fluxgo.Kafka, logger *fluxgo.Logger, handler *handlers.HandlerGetUser) error {
		return mod.CronRoute(cron, "* * * * *", func(ctx context.Context) error {
			logger.Infoln("Cron executed")
			log.Println("Cron executed")

			if err := kafka.ProduceMessageJson(ctx, "TEST", fiber.Map{"foo": "bar"}, nil); err != nil {
				log.Println("Error to produce message", err)
				return err
			}

			return nil
		})
	})
	mod.AddRoute(func(kafka *fluxgo.Kafka, handler *handlers.HandlerGetUser) error {
		return mod.KafkaEvent(kafka, "TEST", func(ctx context.Context, data []byte) error {
			log.Println("Kafka event executed", string(data))
			return nil
		})
	})

	return mod
}
