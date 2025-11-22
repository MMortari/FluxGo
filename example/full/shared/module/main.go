package module

import (
	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user"
	"github.com/MMortari/FluxGo/example/full/shared/http"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
	"github.com/redis/go-redis/v9"
)

func Module() *fluxgo.FluxGo {
	flux := fluxgo.New(fluxgo.FluxGo{Name: "Teste Full", Version: "1", Env: "development", Debugger: true})
	flux.AddApm(fluxgo.ApmOptions{CollectorURL: "localhost:4317", Exporter: "grpc"})
	flux.ConfigLogger(fluxgo.LoggerOptions{Type: "file", Level: "debug", LogFilePath: "full/logs/out.log"})

	flux.AddDatabase(fluxgo.DatabaseOptions{Dsn: "postgres://postgres:postgres@localhost:5435/postgres?sslmode=disable"})
	flux.AddRedis(fluxgo.RedisOptions{Options: redis.Options{Addr: "localhost:6398"}})
	flux.AddCron()
	flux.AddHttp(http.GetHttp(flux.GetApm()))

	flux.AddDependency(repositories.UserRepositoryStart)

	flux.AddModule(user.Module())

	return flux
}
