package handlers

import (
	c "context"
	"encoding/json"
	"errors"
	"log"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/dto"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
	"github.com/gofiber/fiber/v2"
)

type HandlerListUser struct {
	repository *repositories.UserRepository
	tools      *fluxgo.Tools
	prom       *fluxgo.Prometheus
	metrics    *fluxgo.Metrics
	logger     *fluxgo.Logger
	http       *fluxgo.Http
}

func HandlerListUserStart(repository *repositories.UserRepository, tools *fluxgo.Tools, prom *fluxgo.Prometheus, metrics *fluxgo.Metrics, http *fluxgo.Http, logger *fluxgo.Logger) *HandlerListUser {
	return &HandlerListUser{
		repository: repository,
		tools:      tools,
		prom:       prom,
		metrics:    metrics,
		logger:     logger,
		http:       http,
	}
}

func (h *HandlerListUser) Execute(ctx c.Context, data *dto.ListUserReq) (*dto.ListUserRes, *fluxgo.GlobalError) {
	users, err := h.repository.ListUser(ctx, &repositories.UserFilter{IdUser: data.IdUser, Name: data.Name})
	if err != nil {
		return nil, fluxgo.ErrorInternalError("Error to list user")
	}

	return &dto.ListUserRes{Data: users}, nil
}

func (h *HandlerListUser) Name() string {
	return "HandlerListUser"
}
func (h *HandlerListUser) Description() string {
	return "Tool to list user information"
}
func (h *HandlerListUser) Schema() fluxgo.ToolsSchema {
	return fluxgo.ToolParseSchema(dto.ListUserReq{})
}
func (h *HandlerListUser) ExecuteTool(ctx c.Context, raw json.RawMessage) (json.RawMessage, error) {
	resp := &dto.ListUserReq{}
	if err := json.Unmarshal(raw, resp); err != nil {
		return nil, err
	}

	res, err := h.Execute(ctx, resp)
	if err != nil {
		return nil, errors.New(err.Message)
	}

	return json.Marshal(res)
}

func (h *HandlerListUser) HandleCron(ctx c.Context) error {
	h.logger.Info("Cron executed")
	log.Println("Cron executed")
	return nil
}

func (h *HandlerListUser) HandleMessage(ctx c.Context, data []byte) error {
	log.Println("New message on event: " + string(data))
	return nil
}
func (h *HandlerListUser) HandleHttp(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
	resp, err := h.Execute(c.UserContext(), income.(*dto.ListUserReq))
	if err != nil {
		return nil, err
	}
	return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
}
