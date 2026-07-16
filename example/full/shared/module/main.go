package module

import (
	"context"
	"os"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/config"
	"github.com/MMortari/FluxGo/example/full/modules/user"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

func Module() *fluxgo.FluxGo {
	envFile := ".env.development"
	if os.Getenv("ENV") == "test" {
		envFile = ".env.test"
	}
	env := fluxgo.ParseEnv[config.Env](fluxgo.EnvOptions{LoadFromFile: fluxgo.Pointer(envFile), Validate: true})

	flux := fluxgo.New(fluxgo.FluxGoConfig{
		Name:         "Teste Full",
		Version:      "1",
		Env:          &env.Env,
		Debugger:     true,
		FullDebugger: false,
		Otel:         &fluxgo.OtelOptions{CollectorURL: env.Apm.CollectorUrl, Exporter: env.Apm.Exporter},
	})
	flux.AddApm()
	flux.AddMetrics()
	flux.ConfigLogger(fluxgo.LoggerOptions{Type: env.Logger.Type, Level: env.Logger.Level, LogFilePath: env.Logger.FilePath})

	prom := flux.AddPrometheus()
	prom.NewCounterVec(prometheus.CounterOpts{
		Name: "get_user",
		Help: "Quantidade de requests para buscar usuários",
	}, []string{"user"})

	flux.AddDependency(func() *config.Env { return &env })
	flux.AddDatabase(fluxgo.DatabaseOptions{Instances: []fluxgo.DatabaseConn{{Dsn: env.Database.Dsn}}})
	flux.AddRedis(fluxgo.RedisOptions{Options: redis.Options{Addr: env.Redis.Addr}})
	flux.AddKafka(env.Kafka.GetConfig())
	flux.AddCron()
	flux.AddHttp(fluxgo.HttpOptions{Port: 3333, LogRequest: true, AddHealthRoutes: true, Permissions: getPermissions()}, func(data fluxgo.HttpConfigData) {
		data.CreateRouter("/public", middlewareExample())
		data.CreateRouter("/internal", middlewareExample())
	})
	flux.AddTools()
	flux.AddGrpc(fluxgo.GrpcOptions{Port: 50051, Reflection: true})

	flux.AddDependency(repositories.UserRepositoryStart)

	flux.AddModule(user.Module())

	return flux
}

func middlewareExample() fiber.Handler {
	return func(c *fiber.Ctx) error {
		newCtx := context.WithValue(c.UserContext(), fluxgo.RoleContextKey, "view")
		c.SetUserContext(newCtx)

		return c.Next()
	}
}

func getPermissions() *fluxgo.Permissions {
	return &fluxgo.Permissions{
		"admin": []fluxgo.PermissionRule{
			{Action: "manage", Subject: "all"},
		},
		"view": []fluxgo.PermissionRule{
			{Action: "read", Subject: "user"},
			{Action: "read", Subject: "bill"},
		},
	}
}
