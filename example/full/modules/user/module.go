package user

import (
	"context"
	"log"
	"time"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/dto"
	"github.com/MMortari/FluxGo/example/full/modules/user/handlers"
)

func Module() *fluxgo.FluxModule {
	mod := fluxgo.Module("user")

	mod.AddHandler(handlers.HandlerGetUserStart)

	mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
		return mod.HttpRoute(f, "/public", "GET", "/user", fluxgo.RouteIncome{
			Entity:   dto.GetUserReq{},
			Cache:    redis,
			CacheTTL: time.Hour,
		}, handler.HandleHttp)
	})
	mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
		return mod.HttpRoute(f, "/public", "GET", "/user/:id_user", fluxgo.RouteIncome{
			Entity:   dto.GetUserReq{},
			Cache:    redis,
			CacheTTL: time.Hour,
		}, handler.HandleHttp)
	})
	mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
		return mod.HttpRoute(f, "/internal", "POST", "/refresh", fluxgo.RouteIncome{
			Entity:          dto.GetUserReq{},
			Cache:           redis,
			CacheInvalidate: []string{"/public/user"},
		}, handler.HandleHttp)
	})
	mod.AddRoute(func(cron *fluxgo.Cron, logger *fluxgo.Logger, kafka *fluxgo.Kafka, handler *handlers.HandlerGetUser) error {
		return mod.CronRoute(cron, "* * * * *", func(ctx context.Context) error {
			logger.Infoln("Cron executed")
			log.Println("Cron executed")

			content := map[string]interface{}{
				"message": "Hello, Kafka!",
				"time":    time.Now().UnixNano(),
			}

			return kafka.ProduceMessageJson(ctx, "TEST", content, nil)
		})
	})
	mod.AddRoute(func(f *fluxgo.FluxGo, tool *fluxgo.Tools, handler *handlers.HandlerGetUser) error {
		return mod.ToolRoute(f, tool, handler)
	})
	mod.AddRoute(func(f *fluxgo.FluxGo, kafka *fluxgo.Kafka, handler *handlers.HandlerGetUser) error {
		return mod.TopicConsume(kafka, "TEST", handler.HandleMessage)
	})

	return mod
}
