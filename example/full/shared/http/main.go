package http

import (
	fluxgo "github.com/MMortari/FluxGo"
	"github.com/gofiber/fiber/v2"
)

func GetHttp(apm *fluxgo.Apm, prom *fluxgo.Prometheus) *fluxgo.Http {
	http := fluxgo.NewHttp(fluxgo.HttpOptions{Port: 3333, LogRequest: true, Apm: apm, Prometheus: prom, AddHealthRoutes: true})

	http.CreateRouter("/public", middlewareExample(apm))
	http.CreateRouter("/internal", middlewareExample(apm))

	return http
}

func middlewareExample(apm *fluxgo.Apm) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}
