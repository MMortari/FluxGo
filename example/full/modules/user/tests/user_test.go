package handlers

import (
	"testing"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/shared/module"
	"github.com/stretchr/testify/assert"
)

func TestSwagger(t *testing.T) {
	fx, app := module.Module().GetTestApp(t)
	defer fx.RequireStart().RequireStop()

	t.Run("should serve swagger UI", func(t *testing.T) {
		status, body := fluxgo.RunTestRequestRaw(app, "GET", "/swagger", nil, nil)
		assert.Equal(t, 200, status)
		assert.Contains(t, string(body), "swagger-ui")
	})

	t.Run("should serve openapi spec", func(t *testing.T) {
		status, spec := fluxgo.RunTestRequest(app, "GET", "/swagger/openapi.json", nil, nil)
		assert.Equal(t, 200, status)

		assert.Equal(t, "3.0.3", spec["openapi"])
		info := fluxgo.ConvertToMap(spec["info"])
		assert.NotEmpty(t, info["title"])
		assert.Equal(t, "API de exemplo do FluxGo", info["description"])

		paths := fluxgo.ConvertToMap(spec["paths"])
		assert.Contains(t, paths, "/public/user")
		assert.Contains(t, paths, "/public/user/{id_user}")
		assert.Contains(t, paths, "/internal/refresh")

		// GET /public/user tem tag "user"
		get := fluxgo.ConvertToMap(fluxgo.ConvertToMap(paths["/public/user"])["get"])
		tags := get["tags"].([]interface{})
		assert.Equal(t, "user", tags[0])

		// POST /internal/refresh existe
		assert.NotNil(t, fluxgo.ConvertToMap(paths["/internal/refresh"])["post"])

		// /public/user/{id_user} tem path param id_user
		getParam := fluxgo.ConvertToMap(fluxgo.ConvertToMap(paths["/public/user/{id_user}"])["get"])
		params := getParam["parameters"].([]interface{})
		assert.Len(t, params, 1)
		firstParam := fluxgo.ConvertToMap(params[0])
		assert.Equal(t, "id_user", firstParam["name"])
		assert.Equal(t, "path", firstParam["in"])
	})
}

func TestGetUser(t *testing.T) {
	fx, app := module.Module().GetTestApp(t)
	defer fx.RequireStart().RequireStop()

	t.Run("GET /public/user", func(t *testing.T) {
		endpoint := "/public/user"
		successHttpCode := 200

		status, body := fluxgo.RunTestRequest(app, "GET", endpoint, nil, nil)

		assert.Equalf(t, successHttpCode, status, "Invalid status code")
		assert.NotNilf(t, body["data"], "Invalid body response")

		user := fluxgo.ConvertToMap(body["data"].([]interface{})[0])
		assert.Equalf(t, "299f3dcd-42f3-46c1-89d5-603c78a78f50", user["id"], "Invalid body response")
		assert.Equalf(t, "John", user["name"], "Invalid body response")
	})

	t.Run("POST /internal/refresh", func(t *testing.T) {
		endpoint := "/internal/refresh"
		successHttpCode := 200

		status, _ := fluxgo.RunTestRequest(app, "POST", endpoint, nil, nil)

		assert.Equalf(t, successHttpCode, status, "Invalid status code")
	})
}
