package http

import (
	"log"
	"time"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/gofiber/fiber/v2"
)

func GetHttp(apm *fluxgo.Apm) *fluxgo.Http {
	http := fluxgo.NewHttp(fluxgo.HttpOptions{Port: 3333, LogRequest: true, Apm: apm})
	http.CreateRouter("/public", middlewareExample(apm))

	return http
}

func middlewareExample(apm *fluxgo.Apm) fiber.Handler {
	return func(c *fiber.Ctx) error {
		_, span := apm.StartSpan(c.UserContext(), "http/middlewareExample")
		defer span.End()

		// Middleware logic here
		log.Println("MIDDLEWARE: Validating something")

		time.Sleep(time.Second * 1)

		return c.Next()
	}
}
