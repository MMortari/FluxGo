package module

import (
	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
	"github.com/redis/go-redis/v9"
)

func Module() *fluxgo.FluxGo {
	flux := fluxgo.New(fluxgo.FluxGo{Name: "Teste Full", Version: "1", Env: "development", Debugger: true})
	flux.AddApm(fluxgo.ApmOptions{CollectorURL: "localhost:4317", Exporter: "grpc"})

	http := fluxgo.NewHttp(fluxgo.HttpOptions{Port: 3333, LogRequest: true, Apm: flux.GetApm()})
	http.CreateRouter("/public")

	flux.AddDatabase(fluxgo.DatabaseOptions{Dsn: "postgres://postgres:postgres@localhost:5435/postgres?sslmode=disable"})
	flux.AddRedis(fluxgo.RedisOptions{Options: redis.Options{Addr: "localhost:6398"}})
	flux.AddCron()
	flux.AddHttp(http)

	flux.AddDependency(repositories.UserRepositoryStart)

	flux.AddModule(user.Module())

	return flux
}
