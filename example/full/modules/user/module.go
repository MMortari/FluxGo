package user

import (
	"time"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/dto"
	"github.com/MMortari/FluxGo/example/full/modules/user/handlers"
)

func Module() *fluxgo.FluxModule {
	return fluxgo.Module("user").
		AddHandler(
			handlers.HandlerGetUserStart,
			handlers.HandlerListUserStart,
			handlers.HandlerUserGrpcStart,
		).
		Route(
			fluxgo.GET[handlers.HandlerListUser]("/public", "/user", fluxgo.RouteIncome{
				Entity:     dto.ListUserReq{},
				FromQuery:  true,
				CacheTTL:   time.Hour,
				Permission: &fluxgo.RoutePermission{Action: "read", Subject: "user"},
				Doc: &fluxgo.RouteDoc{
					Summary:     "Listagem de usuários",
					Description: "Listagem completa de usuários com filtros opcionais",
					OkResponse:  dto.ListUserRes{},
				},
			}),
			fluxgo.GET[handlers.HandlerGetUser]("/public", "/user/:id_user", fluxgo.RouteIncome{Entity: dto.GetUserReq{}, CacheTTL: time.Hour}),
			fluxgo.POST[handlers.HandlerGetUser]("/internal", "/refresh", fluxgo.RouteIncome{Entity: dto.GetUserReq{}, CacheInvalidate: []string{"/public/user"}}),
			fluxgo.TopicDef[handlers.HandlerGetUser]("TEST"),
			fluxgo.ToolDef[handlers.HandlerGetUser](),
			fluxgo.CronDef[handlers.HandlerGetUser]("* * * * *"),
			fluxgo.GrpcDef[handlers.HandlerUserGrpc](),
		)
}
