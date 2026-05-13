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
	"github.com/prometheus/client_golang/prometheus"
)

type HandlerGetUser struct {
	repository *repositories.UserRepository
	tools      *fluxgo.Tools
	prom       *fluxgo.Prometheus
	logger     *fluxgo.Logger
}

func HandlerGetUserStart(repository *repositories.UserRepository, tools *fluxgo.Tools, prom *fluxgo.Prometheus, logger *fluxgo.Logger) *HandlerGetUser {
	return &HandlerGetUser{repository, tools, prom, logger}
}

func (h *HandlerGetUser) Execute(ctx c.Context, data *dto.GetUserReq) (*dto.GetUserRes, *fluxgo.GlobalError) {
	log := h.logger.CreateLogger(ctx)

	tool, err := h.tools.GetOllamaTools()
	if err != nil {
		return nil, fluxgo.ErrorInternalError("Error to get ollama tools")
	}

	if counter := h.prom.GetCounterVec("counter"); counter != nil {
		counter.With(prometheus.Labels{"user": data.IdUser}).Inc()
	}

	log.Infof("tool: %+v\n\n", tool)

	user, err := h.repository.GetUser(ctx)
	if err != nil {
		return nil, fluxgo.ErrorInternalError("Error to get user")
	}
	if user == nil {
		return nil, fluxgo.ErrorNotFound("User not found")
	}
	return &dto.GetUserRes{User: *user}, nil
}

func (h *HandlerGetUser) Name() string {
	return "HandlerGetUser"
}
func (h *HandlerGetUser) Description() string {
	return "Tool to get user information"
}
func (h *HandlerGetUser) Schema() fluxgo.ToolsSchema {
	return fluxgo.ToolParseSchema(dto.GetUserReq{})
}
func (h *HandlerGetUser) ExecuteTool(ctx c.Context, raw json.RawMessage) (json.RawMessage, error) {
	resp := &dto.GetUserReq{}
	if err := json.Unmarshal(raw, resp); err != nil {
		return nil, err
	}

	res, err := h.Execute(ctx, resp)
	if err != nil {
		return nil, errors.New(err.Message)
	}

	return json.Marshal(res)
}

func (h *HandlerGetUser) HandleCron(ctx c.Context) error {
	h.logger.Info("Cron executed")
	log.Println("Cron executed")
	return nil
}

func (h *HandlerGetUser) HandleMessage(ctx c.Context, data []byte) error {
	log.Println("New message on event: " + string(data))
	return nil
}
func (h *HandlerGetUser) HandleHttp(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
	resp, err := h.Execute(c.UserContext(), income.(*dto.GetUserReq))
	if err != nil {
		return nil, err
	}
	return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
}
