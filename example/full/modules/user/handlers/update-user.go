package handlers

import (
	c "context"
	"encoding/json"
	"errors"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/dto"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
	"github.com/gofiber/fiber/v2"
)

type HandlerUpdateUser struct {
	repository *repositories.UserRepository
}

func HandlerUpdateUserStart(repository *repositories.UserRepository) *HandlerUpdateUser {
	return &HandlerUpdateUser{
		repository: repository,
	}
}

func (h *HandlerUpdateUser) Execute(ctx c.Context, data *dto.UpdateUserReq) (*dto.UpdateUserRes, *fluxgo.GlobalError) {
	user, err := h.repository.GetUser(ctx)
	if err != nil {
		return nil, fluxgo.ErrorInternalError("Error to get user")
	}
	if user == nil {
		return nil, fluxgo.ErrorNotFound("User not found")
	}
	return &dto.UpdateUserRes{Success: true}, nil
}

func (h *HandlerUpdateUser) Name() string {
	return "HandlerUpdateUser"
}
func (h *HandlerUpdateUser) Description() string {
	return "Tool to update user information"
}
func (h *HandlerUpdateUser) Schema() fluxgo.ToolsSchema {
	return fluxgo.ToolParseSchema(dto.UpdateUserReq{})
}
func (h *HandlerUpdateUser) ExecuteTool(ctx c.Context, raw json.RawMessage) (json.RawMessage, error) {
	resp := &dto.UpdateUserReq{}
	if err := json.Unmarshal(raw, resp); err != nil {
		return nil, err
	}

	res, err := h.Execute(ctx, resp)
	if err != nil {
		return nil, errors.New(err.Message)
	}

	return json.Marshal(res)
}

func (h *HandlerUpdateUser) HandleHttp(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
	resp, err := h.Execute(c.UserContext(), income.(*dto.UpdateUserReq))
	if err != nil {
		return nil, err
	}
	return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
}
