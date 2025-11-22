package module

import (
	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user"
	"github.com/MMortari/FluxGo/example/full/shared/http"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
	"github.com/redis/go-redis/v9"
)

type Env struct {
	Database struct {
		Dsn string `env:"DATABASE_DSN" envDefault:"postgres://postgres:postgres@localhost:5435/postgres?sslmode=disable"`
	}
	Redis struct {
		Addr string `env:"REDIS_ADDR" envDefault:"localhost:6398"`
	}
	Logger struct {
		Type     string `env:"LOGGER_TYPE" envDefault:"file"`
		Level    string `env:"LOGGER_LEVEL" envDefault:"debug"`
		FilePath string `env:"LOGGER_FILE_PATH" envDefault:"full/logs/out.log"`
	}
	Apm struct {
		Exporter     string `env:"APM_EXPORTER" envDefault:"grpc"`
		CollectorUrl string `env:"APM_COLLECTOR_URL" envDefault:"localhost:4317"`
	}
}

func Module() *fluxgo.FluxGo {
	env := fluxgo.ParseEnv[Env]()

	flux := fluxgo.New(fluxgo.FluxGo{Name: "Teste Full", Version: "1", Env: "development", Debugger: true})
	flux.AddApm(fluxgo.ApmOptions{CollectorURL: env.Apm.CollectorUrl, Exporter: env.Apm.Exporter})
	flux.ConfigLogger(fluxgo.LoggerOptions{Type: env.Logger.Type, Level: env.Logger.Level, LogFilePath: env.Logger.FilePath})

	flux.AddDependency(func() *Env { return &env })
	flux.AddDatabase(fluxgo.DatabaseOptions{Dsn: env.Database.Dsn})
	flux.AddRedis(fluxgo.RedisOptions{Options: redis.Options{Addr: env.Redis.Addr}})
	flux.AddCron()
	flux.AddHttp(http.GetHttp(flux.GetApm()))

	flux.AddDependency(repositories.UserRepositoryStart)

	flux.AddModule(user.Module())

	return flux
}
