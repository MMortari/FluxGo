package user

import (
	"time"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/dto"
	"github.com/MMortari/FluxGo/example/full/modules/user/handlers"
)

func Module() *fluxgo.FluxModule {
	return fluxgo.Module("user",
		fluxgo.WithSwagger(fluxgo.SwaggerModuleTag{
			Title:       "Usuário",
			Description: "Operações relacionadas a usuários",
		}),
	).
		AddHandler(
			handlers.HandlerGetUserStart,
			handlers.HandlerListUserStart,
			handlers.HandlerUpdateUserStart,
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
			fluxgo.PUT[handlers.HandlerUpdateUser]("/public", "/user/:id_user", fluxgo.RouteIncome{Entity: dto.UpdateUserReq{}, CacheTTL: time.Hour, Doc: &fluxgo.RouteDoc{Summary: "Atualiza informações do usuário", Description: "Atualiza informações do usuário com base no ID fornecido", OkResponse: dto.UpdateUserRes{}}}),
			fluxgo.POST[handlers.HandlerGetUser]("/internal", "/refresh", fluxgo.RouteIncome{Entity: dto.GetUserReq{}, CacheInvalidate: []string{"/public/user"}}),
			fluxgo.TopicDef[handlers.HandlerGetUser]("TEST"),
			fluxgo.ToolDef[handlers.HandlerGetUser](),
			fluxgo.CronDef[handlers.HandlerGetUser]("* * * * *"),
			fluxgo.GrpcDef[handlers.HandlerUserGrpc](),
		)
}
