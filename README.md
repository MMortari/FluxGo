# FluxGo

A Golang framework for fast development reliable applications.

## Example

Flux was created to easily create applications with pre-built capability, like:

- APM with OpenTelemetry
- Log management
- HTTP with Fiber
- Redis with go-redis
- Relational Database with sqlx

For more examples, acesses this [link](https://github.com/MMortari/FluxGo/blob/main/example).

```golang
package main

import (
	"context"
	"log"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/gofiber/fiber/v2"
)

func main() {
	flux := fluxgo.New(fluxgo.FluxGo{Name: "FluxGo Simple Example", Debugger: true})
	flux.AddApm(fluxgo.ApmOptions{CollectorURL: "localhost:4317", Exporter: "grpc"})

	http := fluxgo.NewHttp(fluxgo.HttpOptions{Port: 3333, LogRequest: true, Apm: flux.GetApm()})
	http.CreateRouter("/public")

	flux.AddCron()
	flux.AddHttp(http)

	mod := fluxgo.Module("test")
	mod.AddRoute(func(f *fluxgo.FluxGo) error {
		return mod.HttpRoute(f, "/public", "GET", "/", fluxgo.RouteIncome{Entity: TestIncome{}, FromQuery: true, Validate: true}, func(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
			incomeVal := income.(*TestIncome)

			return &fluxgo.GlobalResponse{Content: fiber.Map{
				"message": "Hello, FluxGo!",
				"income":  incomeVal,
			}, Status: 200}, nil
		})
	})
	mod.AddRoute(func(cron *fluxgo.Cron) error {
		return mod.CronRoute(cron, "@every 10s", func(ctx context.Context) error {
			log.Println("Cron has ran")
			return nil
		})
	})

	flux.AddModule(mod)

	flux.Run()
}

type TestIncome struct {
	Name  string `query:"name" json:"name" validate:"required"`
	Email string `query:"email" json:"email" validate:"required,email"`
}
```
