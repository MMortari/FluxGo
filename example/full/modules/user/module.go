package user

import (
	"time"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/dto"
	"github.com/MMortari/FluxGo/example/full/modules/user/handlers"
)

func Module() *fluxgo.FluxModule {
	return fluxgo.Module("user").
		AddHandler(handlers.HandlerGetUserStart).
		Route(
			fluxgo.GET[handlers.HandlerGetUser]("/public", "/user", fluxgo.RouteIncome{
				Entity:   dto.GetUserReq{},
				CacheTTL: time.Hour,
			}),
			fluxgo.GET[handlers.HandlerGetUser]("/public", "/user/:id_user", fluxgo.RouteIncome{
				Entity:   dto.GetUserReq{},
				CacheTTL: time.Hour,
			}),
			fluxgo.POST[handlers.HandlerGetUser]("/internal", "/refresh", fluxgo.RouteIncome{
				Entity:          dto.GetUserReq{},
				CacheInvalidate: []string{"/public/user"},
			}),
			fluxgo.TopicDef[handlers.HandlerGetUser]("TEST"),
			fluxgo.ToolDef[handlers.HandlerGetUser](),
			fluxgo.CronDef[handlers.HandlerGetUser]("* * * * *"),
		)
}
