package handlers

import (
	c "context"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/dto"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
)

type HandlerGetUser struct {
	repository *repositories.UserRepository
}

func HandlerGetUserStart(repository *repositories.UserRepository) *HandlerGetUser {
	return &HandlerGetUser{repository: repository}
}

func (h *HandlerGetUser) Execute(ctx c.Context, idUser string) (*dto.GetUserRes, *fluxgo.GlobalError) {
	user, err := h.repository.GetUser(ctx)
	if err != nil {
		return nil, fluxgo.ErrorInternalError("Error to get user")
	}
	if user == nil {
		return nil, fluxgo.ErrorNotFound("User not found")
	}
	return &dto.GetUserRes{User: *user}, nil
}
