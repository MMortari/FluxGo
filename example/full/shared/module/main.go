package module

import (
	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/config"
	"github.com/MMortari/FluxGo/example/full/modules/user"
	"github.com/MMortari/FluxGo/example/full/shared/http"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
	"github.com/redis/go-redis/v9"
)

func Module() *fluxgo.FluxGo {
	env := fluxgo.ParseEnv[config.Env](fluxgo.EnvOptions{LoadFromFile: fluxgo.Pointer(".env.development"), Validate: true})

	flux := fluxgo.New(fluxgo.FluxGoConfig{Name: "Teste Full", Version: "1", Env: &env.Env, Debugger: true})
	flux.AddApm(fluxgo.ApmOptions{CollectorURL: env.Apm.CollectorUrl, Exporter: env.Apm.Exporter})
	flux.ConfigLogger(fluxgo.LoggerOptions{Type: env.Logger.Type, Level: env.Logger.Level, LogFilePath: env.Logger.FilePath})

	flux.AddDependency(func() *config.Env { return &env })
	flux.AddDatabase(fluxgo.DatabaseOptions{Dsn: env.Database.Dsn})
	flux.AddRedis(fluxgo.RedisOptions{Options: redis.Options{Addr: env.Redis.Addr}})
	flux.AddCron()
	flux.AddHttp(http.GetHttp(flux.GetApm()))

	flux.AddDependency(repositories.UserRepositoryStart)

	flux.AddModule(user.Module())

	return flux
}
