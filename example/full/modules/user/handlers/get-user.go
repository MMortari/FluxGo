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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type HandlerGetUser struct {
	repository  *repositories.UserRepository
	tools       *fluxgo.Tools
	prom        *fluxgo.Prometheus
	metrics     *fluxgo.Metrics
	logger      *fluxgo.Logger
	http        *fluxgo.Http
	getUserReqs metric.Int64Counter
}

func HandlerGetUserStart(repository *repositories.UserRepository, tools *fluxgo.Tools, prom *fluxgo.Prometheus, metrics *fluxgo.Metrics, http *fluxgo.Http, logger *fluxgo.Logger) *HandlerGetUser {
	return &HandlerGetUser{
		repository:  repository,
		tools:       tools,
		prom:        prom,
		metrics:     metrics,
		logger:      logger,
		http:        http,
		getUserReqs: metrics.NewIntCounter("get_user_requests_total", "Total de requests para buscar usuários"),
	}
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

	h.getUserReqs.Add(ctx, 1, metric.WithAttributes(attribute.String("user", data.IdUser)))

	log.Infof("tool: %+v\n\n", tool)

	user, err := h.repository.GetUser(ctx)
	if err != nil {
		return nil, fluxgo.ErrorInternalError("Error to get user")
	}
	if user == nil {
		return nil, fluxgo.ErrorNotFound("User not found")
	}
	return &dto.GetUserRes{User: *user, Permissions: h.http.GetPermissions(ctx)}, nil
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
